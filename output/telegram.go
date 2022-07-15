package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/tools/vendors"
	"github.com/devopsext/utils"
	"github.com/prometheus/alertmanager/template"
	"golang.org/x/time/rate"
)

type TelegramOutputOptions struct {
	vendors.TelegramOptions
	Message         string
	BotSelector     string
	AlertExpression string
	Forward         string
	RateLimit       int
}

type TelegramOutput struct {
	wg          *sync.WaitGroup
	telegram    *vendors.Telegram
	message     *toolsRender.TextTemplate
	selector    *toolsRender.TextTemplate
	grafana     *render.GrafanaRender
	options     TelegramOutputOptions
	outputs     *common.Outputs
	tracer      sreCommon.Tracer
	logger      sreCommon.Logger
	requests    sreCommon.Counter
	errors      sreCommon.Counter
	rateLimiter *rate.Limiter
}

func (t *TelegramOutput) Name() string {
	return "Telegram"
}

// assume that url is => https://api.telegram.org/botID:botToken/sendMessage?chat_id=%s
func (t *TelegramOutput) getBotID(IDToken string) string {

	arr := strings.SplitN(IDToken, ":", 2)
	if len(arr) == 2 {
		return arr[0]
	}
	return ""
}

func (t *TelegramOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message string) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	if err := t.rateLimiter.Wait(context.TODO()); err != nil {
		return nil, err
	}

	telegramOpts := vendors.TelegramOptions{
		IDToken:               IDToken,
		ChatID:                chatID,
		Timeout:               t.options.Timeout,
		Insecure:              false,
		ParseMode:             t.options.ParseMode,
		DisableNotification:   t.options.DisableNotification,
		DisableWebPagePreview: true,
	}

	messageOpts := vendors.TelegramMessageOptions{
		Text: message,
	}

	b, err := t.telegram.CustomSendMessage(telegramOpts, messageOpts)

	if err != nil {
		t.logger.SpanError(span, err)
		t.logger.SpanDebug(span, message)
		return nil, err
	}

	t.logger.SpanDebug(span, "Response from Telegram => %s", string(b))
	return b, err
}

func (t *TelegramOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message string, err error) error {

	_, e := t.sendMessage(spanCtx, IDToken, chatID, fmt.Sprintf("%s\n%s", message, err.Error()))
	return e
}

func (t *TelegramOutput) sendPhoto(spanCtx sreCommon.TracerSpanContext, IDToken, chatID, message, fileName string, photo []byte) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	telegramOpts := vendors.TelegramOptions{
		IDToken:               IDToken,
		ChatID:                chatID,
		Timeout:               t.options.Timeout,
		Insecure:              false,
		ParseMode:             t.options.ParseMode,
		DisableNotification:   t.options.DisableNotification,
		DisableWebPagePreview: true,
	}

	photoOpts := vendors.TelegramPhotoOptions{
		Caption: message,
		Name:    fileName,
		Content: string(photo),
	}

	return t.telegram.CustomSendPhoto(telegramOpts, photoOpts)
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
		err := errors.New("no alert expression")
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

	if utils.IsEmpty(t.options.Forward) {
		return
	}

	if utils.Contains(event.Via, t.Name()) {
		return
	}

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
	via[t.Name()] = obj

	e := common.Event{
		Time:    event.Time,
		Channel: event.Channel,
		Type:    event.Type,
		Data:    event.Data,
		Via:     via,
	}
	e.SetLogger(t.logger)
	e.SetSpanContext(span.GetContext())

	t.outputs.SendForward(&e, []common.Output{t}, t.options.Forward)
}

func (t *TelegramOutput) Send(event *common.Event) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

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
			b, err := t.selector.RenderObject(jsonObject)
			if err != nil {
				t.logger.SpanDebug(span, err)
			} else {
				IDTokenChatIDs = strings.TrimSpace(string(b))
			}
		}

		if utils.IsEmpty(IDTokenChatIDs) {
			t.logger.SpanDebug(span, "Telegram ID token or chat ID are not found. Skipped")
			return
		}

		b, err := t.message.RenderObject(jsonObject)
		if err != nil {
			t.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			t.logger.SpanDebug(span, "Telegram message is empty")
			return
		}

		t.logger.SpanDebug(span, "Telegram message => %s", message)

		list := strings.Split(IDTokenChatIDs, "\n")
		for _, IDTokenChatID := range list {

			arr := strings.SplitN(IDTokenChatID, "=", 2)
			if len(arr) != 2 {
				continue
			}
			if utils.IsEmpty(arr[0]) || utils.IsEmpty(arr[1]) {
				continue
			}

			IDToken := arr[0]
			chatID := arr[1]
			botID := t.getBotID(IDToken)

			t.requests.Inc(botID, chatID)

			switch event.Type {
			case "AlertmanagerEvent":
				bytes, err := t.sendAlertmanagerImage(span.GetContext(), IDToken, chatID, message, event.Data.(template.Alert))
				if err != nil {
					t.errors.Inc(botID, chatID)
					t.sendErrorMessage(span.GetContext(), IDToken, chatID, message, err)
				} else {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				bytes, err := t.sendMessage(span.GetContext(), IDToken, chatID, message)

				if err != nil {
					t.errors.Inc(botID, chatID)
				} else {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func NewTelegramOutput(wg *sync.WaitGroup,
	options TelegramOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *TelegramOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.Message) {
		logger.Debug("Telegram message is not defined. Skipped")
		return nil
	}

	telegram, err := vendors.NewTelegram(options.TelegramOptions)
	if err != nil {
		logger.Error(err)
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "telegram-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts)
	if err != nil {
		logger.Error(err)
		return nil
	}

	selectorOpts := toolsRender.TemplateOptions{
		Name:       "telegram-selector",
		Content:    common.Content(options.BotSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	selector, err := toolsRender.NewTextTemplate(selectorOpts)
	if err != nil {
		logger.Error(err)
	}

	return &TelegramOutput{
		rateLimiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(options.RateLimit)), 1),
		wg:          wg,
		telegram:    telegram,
		message:     message,
		selector:    selector,
		grafana:     render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:     options,
		outputs:     outputs,
		logger:      logger,
		tracer:      observability.Traces(),
		requests:    observability.Metrics().Counter("requests", "Count of all telegram requests", []string{"bot_id", "chat_id"}, "telegram", "output"),
		errors:      observability.Metrics().Counter("errors", "Count of all telegram errors", []string{"bot_id", "chat_id"}, "telegram", "output"),
	}
}
