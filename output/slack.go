package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/processor"

	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/tools/vendors"
	"github.com/devopsext/utils"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/alertmanager/template"
)

type SlackOutputOptions struct {
	Timeout         int
	Token           string
	Channel         string
	Message         string
	ChannelSelector string
	AlertExpression string
	Forward         string
	Insecure        bool
}

type SlackOutput struct {
	wg       *sync.WaitGroup
	slack    *vendors.Slack
	message  *toolsRender.TextTemplate
	selector *toolsRender.TextTemplate
	grafana  *render.GrafanaRender
	options  SlackOutputOptions
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

func (s *SlackOutput) Name() string {
	return "Slack"
}

func waitDDImage(url string, timeout int) bool {
	if timeout <= 0 {
		timeout = 3
	}
	for i := 0; i < timeout; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 && resp.ContentLength > 179 {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func (s *SlackOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, msg vendors.SlackMessageOptions) ([]byte, error) {
	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	s.logger.Debug("%+v", msg)

	message := strings.TrimSpace(msg.Text)
	if utils.IsEmpty(message) {
		err := errors.New("no slack message")
		s.logger.SpanDebug(span, err.Error())
		return nil, err
	}

	b, err := s.slack.SendMessage(msg)
	if err != nil {
		s.logger.SpanError(span, err)
		return nil, err
	}

	s.logger.SpanDebug(span, "Response from Slack => %s", string(b))
	return b, nil
}

func (s *SlackOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, channel string, err error) error {
	fileOpts := vendors.SlackFileOptions{
		Channel: channel,
		Title:   "Error",
		Text:    "Error occurred during processing",
		Name:    "error.txt",
		Content: err.Error(),
		Type:    "text",
	}
	_, e := s.slack.SendFile(fileOpts)
	return e
}

func (s *SlackOutput) sendImage(spanCtx sreCommon.TracerSpanContext, channel, message, fileName, title string, image []byte) ([]byte, error) {
	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	fileOpts := vendors.SlackFileOptions{
		Channel: channel,
		Text:    message,
		Name:    fileName,
		Title:   title,
		Content: string(image),
	}

	return s.slack.SendFile(fileOpts)
}

func (s *SlackOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, channel, message string, alert template.Alert) ([]byte, error) {
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
		msgOptions := vendors.SlackMessageOptions{
			Channel: channel,
			Title:   query,
			Text:    message,
		}
		return s.sendMessage(span.GetContext(), msgOptions)
	}

	image, fileName, err := s.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		s.sendErrorMessage(span.GetContext(), channel, err)
		return nil, nil
	}
	return s.sendImage(span.GetContext(), channel, message, fileName, query, image)
}

func (s *SlackOutput) sendGlobally(spanCtx sreCommon.TracerSpanContext, event *common.Event, bytes []byte) {
	if utils.IsEmpty(s.options.Forward) {
		return
	}

	if common.InterfaceContains(event.Via, s.Name()) {
		s.logger.Debug("Event has been sent already")
		return
	}

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
	via[s.Name()] = obj

	e := common.Event{
		Time:    event.Time,
		Channel: event.Channel,
		Type:    event.Type,
		Data:    event.Data,
		Via:     via,
	}
	e.SetLogger(s.logger)
	e.SetSpanContext(span.GetContext())

	s.outputs.SendForward(&e, []common.Output{s}, s.options.Forward)
}

func (s *SlackOutput) Send(event *common.Event) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

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

		if common.InterfaceContains(event.Via, s.Name()) {
			s.logger.Debug("Event has been sent already")
			return
		}

		jsonMap, err := event.JsonMap()
		if err != nil {
			s.logger.SpanError(span, err)
			return
		}

		channel := s.options.Channel
		var chans []string
		if s.selector != nil {
			b, err := s.selector.RenderObject(jsonMap)
			if err != nil {
				s.logger.SpanDebug(span, err)
			} else {
				chans = strings.Split(string(b), "\n")
			}
		} else {
			chans = append(chans, channel)
		}

		if len(chans) == 0 {
			s.logger.SpanError(span, fmt.Sprintf("slack no channels for %s", event.Type))
			return
		}

		b, err := s.message.RenderObject(jsonMap)
		if err != nil {
			s.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			s.logger.SpanDebug(span, "Slack message is empty")
			return
		}

		s.logger.SpanDebug(span, "Slack message => %s", message)

		for _, ch := range chans {
			ch = strings.TrimSpace(ch)

			s.requests.Inc(ch)

			switch event.Type {
			case "AlertmanagerEvent":
				msgOptions := vendors.SlackMessageOptions{
					Channel: ch,
					Title:   "AlertmanagerEvent",
					Text:    message,
				}
				bytes, err := s.sendMessage(span.GetContext(), msgOptions)
				if err != nil {
					s.errors.Inc(ch)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			case "DataDogEvent":
				var slackMsg struct {
					Text        string `json:"text"`
					Title       string `json:"title"`
					ImageURL    string `json:"image_url,omitempty"`
					Attachments string `json:"attachments,omitempty"`
					Blocks      string `json:"blocks,omitempty"`
					Thread      string `json:"thread,omitempty"`
				}

				err = json.Unmarshal([]byte(message), &slackMsg)
				if err != nil {
					s.errors.Inc(ch)
					s.logger.SpanError(span, err)
					return
				}

				msgOptions := vendors.SlackMessageOptions{
					Channel:     ch,
					Title:       slackMsg.Title,
					Text:        slackMsg.Text,
					Thread:      slackMsg.Thread,
					Attachments: slackMsg.Attachments,
					Blocks:      slackMsg.Blocks,
				}

				bytes, err := s.sendMessage(span.GetContext(), msgOptions)
				if err != nil {
					s.errors.Inc(ch)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			case "ZabbixEvent":
				jData, err := json.Marshal(event.Data)
				if err != nil {
					break
				}
				var data processor.ZabbixEvent
				err = json.Unmarshal(jData, &data)
				if err != nil {
					break
				}

				color := "#888888"
				switch data.Status {
				case "RESOLVED", "OK":
					color = "#008800"
				case "PROBLEM", "ERROR", "CRITICAL":
					color = "#880000"
				}

				// Create attachment with color
				attachment := fmt.Sprintf(`[{"color": "%s", "text": "%s"}]`, color, message)

				msgOptions := vendors.SlackMessageOptions{
					Channel:     ch,
					Title:       "Zabbix Event",
					Text:        message,
					Attachments: attachment,
				}

				bytes, err := s.sendMessage(span.GetContext(), msgOptions)
				if err != nil {
					s.errors.Inc(ch)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				msgOptions := vendors.SlackMessageOptions{
					Channel: ch,
					Title:   "",
					Text:    message,
				}
				bytes, err := s.sendMessage(span.GetContext(), msgOptions)
				if err != nil {
					s.errors.Inc(ch)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func NewSlackOutput(wg *sync.WaitGroup,
	options SlackOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *SlackOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.Message) {
		logger.Debug("Slack message is not defined. Skipped")
		return nil
	}

	slackOpts := vendors.SlackOptions{
		Timeout:  options.Timeout,
		Token:    options.Token,
		Insecure: options.Insecure,
	}

	slack := vendors.NewSlack(slackOpts)

	messageOpts := toolsRender.TemplateOptions{
		Name:       "slack-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	selectorOpts := toolsRender.TemplateOptions{
		Name:       "slack-selector",
		Content:    common.Content(options.ChannelSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	selector, err := toolsRender.NewTextTemplate(selectorOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &SlackOutput{
		wg:       wg,
		slack:    slack,
		message:  message,
		selector: selector,
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all slack requests", []string{"channel"}, "slack", "output"),
		errors:   observability.Metrics().Counter("errors", "Count of all slack errors", []string{"channel"}, "slack", "output"),
	}
}
