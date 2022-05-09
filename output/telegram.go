package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	vendors "github.com/devopsext/tools/vendors"
	"github.com/devopsext/utils"

	"github.com/prometheus/alertmanager/template"
)

/*type TelegramOutputOptions struct {
	Message             string
	URLSelector         string
	URL                 string
	Timeout             int
	AlertExpression     string
	DisableNotification string
	*vendors.TelegramOptions
}*/

type TelegramOutputOptions struct {
	vendors.TelegramOptions
	Message         string
	BotSelector     string
	AlertExpression string
}

type TelegramOutput struct {
	wg       *sync.WaitGroup
	telegram *vendors.Telegram
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.GrafanaRender
	options  TelegramOutputOptions
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	counter  sreCommon.Counter
}

// assume that url is => https://api.telegram.org/botID:botToken/sendMessage?chat_id=%s
func (t *TelegramOutput) getBotID(IDToken string) string {

	arr := strings.Split(IDToken, ":")
	if len(arr) > 0 {
		return arr[0]
	}
	return ""
}

func (t *TelegramOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message string) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	b, err := t.telegram.SendCustomMessage(vendors.TelegramOptions{
		IDToken:               IDToken,
		ChatID:                chatID,
		Timeout:               t.options.Timeout,
		Insecure:              false,
		DisableNotification:   t.options.DisableNotification,
		DisableWebPagePreview: true,
		MessageOptions: &vendors.TelegramMessageOptions{
			Text: message,
		},
	})

	if err != nil {
		t.logger.SpanError(span, err)
		return nil, err
	}

	t.logger.SpanDebug(span, "Response from Telegram => %s", string(b))
	t.counter.Inc(t.getBotID(IDToken), chatID)
	return b, err
}

func (t *TelegramOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message string, err error) error {

	_, e := t.sendMessage(spanCtx, IDToken, chatID, fmt.Sprintf("%s\n%s", message, err.Error()))
	return e
}

func (t *TelegramOutput) sendPhoto(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message, fileName string, photo []byte) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	return t.telegram.SendCustomPhoto(vendors.TelegramOptions{
		IDToken:               IDToken,
		ChatID:                chatID,
		Timeout:               t.options.Timeout,
		Insecure:              false,
		DisableNotification:   t.options.DisableNotification,
		DisableWebPagePreview: true,
		PhotoOptions: &vendors.TelegramPhotoOptions{
			Caption: message,
			Name:    fileName,
			Content: string(photo),
		},
	})
}

func (t *TelegramOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message string, alert template.Alert) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return nil, err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[t.options.AlertExpression]
	if !ok {
		err := errors.New("No alert expression")
		return nil, err
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

	messageQuery := fmt.Sprintf("%s\n<i>%s</i>", message, query)
	if t.grafana == nil {
		return t.sendMessage(span.GetContext(), IDToken, chatID, messageQuery)
	}

	image, fileName, err := t.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		t.sendErrorMessage(span.GetContext(), IDToken, chatID, messageQuery, err)
		return nil, nil
	}
	return t.sendPhoto(span.GetContext(), IDToken, chatID, messageQuery, fileName, image)
}

func (t *TelegramOutput) sendGlobally(spanCtx sreCommon.TracerSpanContext, event *common.Event, bytes []byte) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var obj interface{}
	if err := json.Unmarshal(bytes, &obj); err != nil {
		t.logger.SpanError(span, err)
		return
	}

	via := event.Via
	if via == nil {
		via = make(map[string]interface{})
	}
	via["Telegram"] = obj

	e := common.Event{
		Time:    event.Time,
		Channel: event.Channel,
		Type:    event.Type,
		Data:    event.Data,
		Via:     via,
	}
	e.SetLogger(t.logger)
	e.SetSpanContext(span.GetContext())

	t.outputs.SendExclude(&e, []common.Output{t})
}

func (t *TelegramOutput) Send(event *common.Event) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		if t.telegram == nil || t.message == nil {
			t.logger.Debug("No telegram client or message")
			return
		}

		if event == nil {
			t.logger.Debug("Event is empty")
			return
		}

		span := t.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			t.logger.SpanError(span, "Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			t.logger.SpanError(span, err)
			return
		}

		IDTokenChatIDs := ""
		if !utils.IsEmpty(t.options.IDToken) && !utils.IsEmpty(t.options.ChatID) {
			IDTokenChatIDs = fmt.Sprintf("%s=%s", t.options.IDToken, t.options.ChatID)
		}

		if t.selector != nil {
			b, err := t.selector.Execute(jsonObject)
			if err != nil {
				t.logger.SpanDebug(span, err)
			} else {
				IDTokenChatIDs = b.String()
			}
		}

		if utils.IsEmpty(IDTokenChatIDs) {
			t.logger.SpanError(span, "Telegram ID token or chat ID are not found")
			return
		}

		b, err := t.message.Execute(jsonObject)
		if err != nil {
			t.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			t.logger.SpanDebug(span, "Telegram message is empty")
			return
		}

		t.logger.SpanDebug(span, "Telegram message => %s", message)

		list := strings.Split(IDTokenChatIDs, "\n")
		for _, IDTokenChatID := range list {

			arr := strings.Split(IDTokenChatID, "=")
			if len(arr) < 2 {
				continue
			}
			if utils.IsEmpty(arr[0]) || utils.IsEmpty(arr[1]) {
				continue
			}

			IDToken := arr[0]
			chatID := arr[1]

			switch event.Type {
			case "AlertmanagerEvent":
				bytes, err := t.sendAlertmanagerImage(span.GetContext(), IDToken, chatID, message, event.Data.(template.Alert))
				if err != nil {
					t.sendErrorMessage(span.GetContext(), IDToken, chatID, message, err)
				} else {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				bytes, err := t.sendMessage(span.GetContext(), IDToken, chatID, message)
				if err == nil {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func NewTelegramOutput(wg *sync.WaitGroup,
	options TelegramOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *TelegramOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.Message) {
		logger.Debug("Telegram message is not defined. Skipped")
		return nil
	}

	return &TelegramOutput{
		wg:       wg,
		telegram: vendors.NewTelegram(options.TelegramOptions),
		message:  render.NewTextTemplate("telegram-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("telegram-selector", options.BotSelector, templateOptions, options, logger),
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all telegram requests", []string{"bot_id", "chat_id"}, "telegram", "output"),
	}
}
