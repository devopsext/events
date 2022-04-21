package output

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"net/url"
	"strconv"
	"strings"
	"sync"

	sreCommon "github.com/devopsext/sre/common"
	toolsVendors "github.com/devopsext/tools/vendors"
	"github.com/devopsext/utils"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/alertmanager/template"
)

type SlackOutputOptions struct {
	Timeout         int
	Token           string
	Channels        []string
	Message         string
	ChannelSelector string
	AlertExpression string
}

type SlackOutput struct {
	wg       *sync.WaitGroup
	slack    *toolsVendors.Slack
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.GrafanaRender
	options  SlackOutputOptions
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	counter  sreCommon.Counter
}

// assume that url is => https://slack.com/api/files.upload?token=%s&channels=%s
func (s *SlackOutput) getChannel(URL string) string {

	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}
	return u.Query().Get("channels")
}

func (s *SlackOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, m toolsVendors.SlackMessage) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	b, err := s.slack.SendMessageCustom(m)
	if err != nil {
		s.logger.SpanError(span, err)
		return nil, err
	}

	s.logger.SpanDebug(span, "Response from Slack => %s", string(b))
	s.counter.Inc(m.Channel)
	return b, nil
}

func (s *SlackOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, m toolsVendors.SlackMessage, err error) error {
	m.FileContent = err.Error()
	_, e := s.sendMessage(spanCtx, m)
	return e
}

func (s *SlackOutput) sendImage(spanCtx sreCommon.TracerSpanContext, token, channel, message, fileName, title string, image []byte) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	m := toolsVendors.SlackMessage{
		Token:       token,
		Channel:     channel,
		Message:     message,
		FileName:    fileName,
		Title:       title,
		FileContent: string(image),
	}

	return s.slack.SendCustomFile(m)
}

func (s *SlackOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, token, channel, message string, alert template.Alert) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return nil, err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[s.options.AlertExpression]
	if !ok {
		return nil, fmt.Errorf("no alert expression")
	}

	caption := alert.Labels["alertname"]
	unit := alert.Labels["unit"]

	var minutes *int

	if m, err := strconv.Atoi(alert.Labels["minutes"]); err == nil {
		minutes = &m
	}

	expr, err := metricsql.Parse(query)
	if err != nil {
		return nil, err
	}

	metric := query
	operator := ""
	var value *float64

	binExpr, ok := expr.(*metricsql.BinaryOpExpr)
	if binExpr != nil && ok {
		metric = string(binExpr.Left.AppendString(nil))
		operator = binExpr.Op

		if v, err := strconv.ParseFloat(string(binExpr.Right.AppendString(nil)), 64); err == nil {
			value = &v
		}
	}

	if s.grafana == nil {
		return s.sendMessage(span.GetContext(), toolsVendors.SlackMessage{Token: token, Channel: channel, Message: message, Title: query})
	}

	image, fileName, err := s.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		s.sendErrorMessage(span.GetContext(),
			toolsVendors.SlackMessage{Token: token, Channel: channel, Message: message, Title: query}, err)
		return nil, nil
	}
	return s.sendImage(span.GetContext(), token, channel, message, fileName, query, image)
}

func (s *SlackOutput) sendGlobally(spanCtx sreCommon.TracerSpanContext, event *common.Event, bytes []byte) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var obj interface{}
	if err := json.Unmarshal(bytes, &obj); err != nil {
		s.logger.SpanError(span, err)
		return
	}

	via := event.Via
	if via == nil {
		via = make(map[string]interface{})
	}
	via["Slack"] = obj

	e := common.Event{
		Time:    event.Time,
		Channel: event.Channel,
		Type:    event.Type,
		Data:    event.Data,
		Via:     via,
	}
	e.SetLogger(s.logger)
	e.SetSpanContext(span.GetContext())

	s.outputs.SendExclude(&e, []common.Output{s})
}

func (s *SlackOutput) Send(event *common.Event) {

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if s == nil || s.message == nil {
			s.logger.Debug("No slack client or message")
			return
		}

		if event == nil {
			s.logger.Debug("Event is empty")
			return
		}

		span := s.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			s.logger.SpanError(span, "Event data is empty")
			return
		}

		jsonMap, err := event.JsonMap()
		if err != nil {
			s.logger.SpanError(span, err)
			return
		}

		defChan := s.options.Channels
		defToken := s.options.Token
		var chans []string
		if s.selector != nil {

			b, err := s.selector.Execute(jsonMap)
			if err != nil {
				s.logger.SpanDebug(span, err)
			} else {
				chans = strings.Split(b.String(), "\n")
			}
		}

		if len(chans) == 0 {
			s.logger.SpanError(span, "slack no channels")
			return
		}
		//if utils.IsEmpty(channels) {
		//	s.logger.SpanError(span, "Slack channels are not found")
		//	return
		//}

		b, err := s.message.Execute(jsonMap)
		if err != nil {
			s.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			s.logger.SpanDebug(span, "Slack message is empty")
			return
		}

		s.logger.SpanDebug(span, "Slack message => %s", message)

		for _, ch := range chans {

			ch = strings.TrimSpace(ch)
			chTuple := strings.Split(ch, "=")
			if len(chTuple) != 2 {
				continue
			}

			if chTuple[0] != "" {
				defToken = chTuple[0]
			}

			if chTuple[1] != "" {
				defChan = strings.Split(chTuple[1], ",")
			}

			for _, slackChan := range defChan {
				switch event.Type {
				case "AlertmanagerEvent":
					m := toolsVendors.SlackMessage{
						Token:   defToken,
						Channel: slackChan,
						Message: message,
						Title:   "AlertmanagerEvent",
					}
					bytes, err := s.sendAlertmanagerImage(span.GetContext(), defToken, slackChan, message, event.Data.(template.Alert))
					if err != nil {
						s.sendErrorMessage(span.GetContext(), m, err)
					} else {
						s.sendGlobally(span.GetContext(), event, bytes)
					}
				case "DataDogEvent":
					m := slackMessageFromDDEvent(event)
					m.Token = defToken
					m.Channel = slackChan
					bytes, err := s.sendMessage(span.GetContext(), m)
					if err == nil {
						s.sendGlobally(span.GetContext(), event, bytes)
					}
				default:
					m := toolsVendors.SlackMessage{
						Token:   defToken,
						Channel: slackChan,
						Message: message,
						Title:   "No title",
					}
					bytes, err := s.sendMessage(span.GetContext(), m)
					if err == nil {
						s.sendGlobally(span.GetContext(), event, bytes)
					}
				}
			}
		}
	}()
}

func slackMessageFromDDEvent(e *common.Event) toolsVendors.SlackMessage {
	var m toolsVendors.SlackMessage
	jsonBytes, err := e.JsonBytes()
	if err != nil {
		return m
	}

	m.Title = gjson.GetBytes(jsonBytes, "data.event.title").String()
	m.Message = gjson.GetBytes(jsonBytes, "data.text_only_msg").String()
	m.ImageURL = gjson.GetBytes(jsonBytes, "data.snapshot").String()

	return m
}

func NewSlackOutput(wg *sync.WaitGroup,
	options SlackOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *SlackOutput {

	logger := observability.Logs()
	//if utils.IsEmpty(options.Token) {
	//	logger.Debug("Slack Token is not defined. Skipped")
	//	return nil
	//}
	//
	//if len(options.Channels) < 1 || utils.IsEmpty(options.Channels[0]) {
	//	logger.Debug("Slack Channel is not defined. Skipped")
	//	return nil
	//}

	return &SlackOutput{
		wg: wg,
		slack: toolsVendors.NewSlack(toolsVendors.SlackOptions{
			Timeout: options.Timeout,
		}),
		message:  render.NewTextTemplate("slack-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("slack-selector", options.ChannelSelector, templateOptions, options, logger),
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all slack requests", []string{"channel"}, "slack", "output"),
	}
}
