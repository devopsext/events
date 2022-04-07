package output

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	sreCommon "github.com/devopsext/sre/common"
	toolsCommon "github.com/devopsext/tools/common"
	messaging "github.com/devopsext/tools/messaging"
	"github.com/devopsext/utils"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/alertmanager/template"
)

type SlackOutputOptions struct {
	URL             string
	Timeout         int
	Message         string
	URLSelector     string
	AlertExpression string
}

type SlackOutput struct {
	wg       *sync.WaitGroup
	slack    toolsCommon.Messenger
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

func (s *SlackOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, URL, message, title, content string) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	b, err := s.slack.SendCustom(URL, message, title, content)
	if err != nil {
		s.logger.SpanError(span, err)
		return nil, err
	}

	s.logger.SpanDebug(span, "Response from Slack => %s", string(b))
	s.counter.Inc(s.getChannel(URL))
	return b, nil
}

func (s *SlackOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, URL, message, title string, err error) error {

	_, e := s.sendMessage(spanCtx, URL, message, title, err.Error())
	return e
}

func (s *SlackOutput) sendImage(spanCtx sreCommon.TracerSpanContext, URL, message, fileName, title string, image []byte) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	return s.slack.SendCustomFile(URL, message, fileName, title, image)
}

func (s *SlackOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, URL, message string, alert template.Alert) ([]byte, error) {

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
		return s.sendMessage(span.GetContext(), URL, message, query, "No image")
	}

	image, fileName, err := s.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		s.sendErrorMessage(span.GetContext(), URL, message, query, err)
		return nil, nil
	}
	return s.sendImage(span.GetContext(), URL, message, fileName, query, image)
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

		if s.slack == nil || s.message == nil {
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

		URLs := s.options.URL
		if s.selector != nil {

			b, err := s.selector.Execute(jsonMap)
			if err != nil {
				s.logger.SpanDebug(span, err)
			} else {
				URLs = b.String()
			}
		}

		if utils.IsEmpty(URLs) {
			s.logger.SpanError(span, "Slack URLs are not found")
			return
		}

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
		arr := strings.Split(URLs, "\n")

		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if utils.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "AlertmanagerEvent":
				bytes, err := s.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert))
				if err != nil {
					s.sendErrorMessage(span.GetContext(), URL, message, "No title", err)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				bytes, err := s.sendMessage(span.GetContext(), URL, message, "No title", "No image")
				if err == nil {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func NewSlackOutput(wg *sync.WaitGroup,
	options SlackOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *SlackOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) {
		logger.Debug("Slack URL is not defined. Skipped")
		return nil
	}

	return &SlackOutput{
		wg: wg,
		slack: messaging.NewSlack(messaging.SlackOptions{
			URL:     options.URL,
			Timeout: options.Timeout,
		}),
		message:  render.NewTextTemplate("slack-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("slack-selector", options.URLSelector, templateOptions, options, logger),
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all slack requests", []string{"channel"}, "slack", "output"),
	}
}
