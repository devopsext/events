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

// assume that url is => https://slack.com/api/files.upload?token=%s&channels=%s
/*func (s *SlackOutput) getChannel(URL string) string {

	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}
	return u.Query().Get("channels")
}*/

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

func (s *SlackOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, m vendors.SlackMessage) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	// check if dd return 1x1 image
	// moved to template by using urlWait

	/*if !utils.IsEmpty(m.ImageURL) && strings.Contains(m.ImageURL, "datadoghq") {
		if !waitDDImage(m.ImageURL, 3) {
			s.logger.SpanDebug(span, "Can't get image from datadoghq: %s", m.ImageURL)
			m.ImageURL = "https://via.placeholder.com/452x185.png?text=No%20chart%20image"
		}
	}*/

	s.logger.Debug("%+v", m)

	msg := strings.TrimSpace(m.Message)
	if utils.IsEmpty(msg) {
		err := errors.New("no slack message")
		s.logger.SpanDebug(span, err.Error())
		return nil, err
	}

	b, err := s.slack.SendCustomMessage(m)
	if err != nil {
		s.logger.SpanError(span, err)
		return nil, err
	}

	s.logger.SpanDebug(span, "Response from Slack => %s", string(b))
	return b, nil
}

func (s *SlackOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, m vendors.SlackMessage, err error) error {
	m.FileContent = err.Error()
	_, e := s.sendMessage(spanCtx, m)
	return e
}

func (s *SlackOutput) sendImage(spanCtx sreCommon.TracerSpanContext, token, channel, message, fileName, title string, image []byte) ([]byte, error) {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	m := vendors.SlackMessage{
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
		return s.sendMessage(span.GetContext(), vendors.SlackMessage{Token: token, Channel: channel, Message: message, Title: query})
	}

	image, fileName, err := s.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		s.sendErrorMessage(span.GetContext(),
			vendors.SlackMessage{Token: token, Channel: channel, Message: message, Title: query}, err)
		return nil, nil
	}
	return s.sendImage(span.GetContext(), token, channel, message, fileName, query, image)
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
		token := s.options.Token
		var chans []string
		if s.selector != nil {
			b, err := s.selector.RenderObject(jsonMap)
			if err != nil {
				s.logger.SpanDebug(span, err)
			} else {
				chans = strings.Split(string(b), "\n")
			}
		} else {
			chans = append(chans, fmt.Sprintf("%s=%s", token, channel))
		}

		if len(chans) == 0 {
			s.logger.SpanError(span, "slack no channels")
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
			chTuple := strings.SplitN(ch, "=", 2)
			if len(chTuple) != 2 {
				continue
			}

			if chTuple[0] != "" {
				token = chTuple[0]
			}

			if chTuple[1] != "" {
				channel = chTuple[1]
			}

			s.requests.Inc(channel)

			switch event.Type {
			case "AlertmanagerEvent":
				m := vendors.SlackMessage{
					Token:   token,
					Channel: channel,
					Message: message,
					Title:   "AlertmanagerEvent",
				}
				// TODO: improve sendAlertmanagerImage to be compatible with slack output
				//bytes, err := s.sendAlertmanagerImage(span.GetContext(), token, channel, message, event.Data.(template.Alert))
				bytes, err := s.sendMessage(span.GetContext(), m)
				if err != nil {
					s.errors.Inc(channel)
					//s.sendErrorMessage(span.GetContext(), m, err)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			case "DataDogEvent":
				var m vendors.SlackMessage
				err = json.Unmarshal([]byte(message), &m)
				if err != nil {
					s.errors.Inc(channel)
					s.logger.SpanError(span, err)
					return
				}
				m.Token = token
				m.Channel = channel
				bytes, err := s.sendMessage(span.GetContext(), m)
				if err != nil {
					s.errors.Inc(channel)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				m := prepareSlackMessage(token, channel, "", message)
				bytes, err := s.sendMessage(span.GetContext(), m)
				if err != nil {
					s.errors.Inc(channel)
				} else {
					s.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func prepareSlackMessage(token string, channel string, title string, message string) vendors.SlackMessage {
	if utils.IsEmpty(title) && !utils.IsEmpty(message) {
		delim := "\n"
		lines := strings.Split(message, delim)
		for i, line := range lines {
			if !utils.IsEmpty(line) {
				title = strings.ReplaceAll(line, "*", "") // no stars in title
				message = "no message"
				if i < len(lines) {
					message = strings.Join(lines[i+1:], delim)
				}
				break
			}
		}
		if utils.IsEmpty(title) {
			title = "no title"
		}
	}

	return vendors.SlackMessage{
		Token:   token,
		Channel: channel,
		Message: message,
		Title:   title,
	}
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

	slack, err := vendors.NewSlack(vendors.SlackOptions{
		Timeout: options.Timeout,
	})
	if err != nil {
		logger.Error(err)
		return nil
	}

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
