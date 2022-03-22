package output

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"

	"github.com/prometheus/alertmanager/template"
)

type TelegramOutputOptions struct {
	Message             string
	URLSelector         string
	URL                 string
	Timeout             int
	AlertExpression     string
	DisableNotification string
}

type TelegramOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.GrafanaRender
	options  TelegramOutputOptions
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	counter  sreCommon.Counter
}

// assume that url is => https://api.telegram.org/botID:botToken/sendMessage?chat_id=%s

func (t *TelegramOutput) getBotID(URL string) string {

	arr := strings.Split(URL, "/bot")
	if len(arr) > 1 {
		arr = strings.Split(arr[1], ":")
		if len(arr) > 0 {
			return arr[0]
		}
	}
	return ""
}

func (t *TelegramOutput) getBotToken(URL string) string {

	arr := strings.Split(URL, "/bot")
	if len(arr) > 1 {
		arr = strings.Split(arr[1], ":")
		if len(arr) > 1 {
			return arr[1]
		}
	}
	return ""
}

func (t *TelegramOutput) getChatID(URL string) string {

	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}
	return u.Query().Get("chat_id")
}

func (t *TelegramOutput) getSendPhotoURL(URL string) string {

	return strings.Replace(URL, "sendMessage", "sendPhoto", -1)
}

func (t *TelegramOutput) post(spanCtx sreCommon.TracerSpanContext, URL, contentType string, body bytes.Buffer, message string) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	t.logger.SpanDebug(span, "Post to Telegram (%s) => %s", URL, message)
	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		t.logger.SpanError(span, err)
		return err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.SpanError(span, err)
		return err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.logger.SpanError(span, err)
		return err
	}

	t.logger.SpanDebug(span, "Response from Telegram => %s", string(b))
	t.counter.Inc(t.getChatID(URL))

	return nil
}

func (t *TelegramOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, URL, message string) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			t.logger.SpanWarn(span, "Failed to close writer")
		}
	}()

	if err := w.WriteField("text", message); err != nil {
		return err
	}

	if err := w.WriteField("parse_mode", "HTML"); err != nil {
		return err
	}

	if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
		return err
	}

	if err := w.WriteField("disable_notification", t.options.DisableNotification); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return t.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (t *TelegramOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, URL, message string, err error) error {
	return t.sendMessage(spanCtx, URL, fmt.Sprintf("%s\n%s", message, err.Error()))
}

func (t *TelegramOutput) sendPhoto(spanCtx sreCommon.TracerSpanContext, URL, message, fileName string, photo []byte) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			t.logger.SpanWarn(span, "Failed to close writer")
		}
	}()

	if err := w.WriteField("caption", message); err != nil {
		return err
	}

	if err := w.WriteField("parse_mode", "HTML"); err != nil {
		return err
	}

	if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
		return err
	}

	if err := w.WriteField("disable_notification", t.options.DisableNotification); err != nil {
		return err
	}

	fw, err := w.CreateFormFile("photo", fileName)
	if err != nil {
		return err
	}

	if _, err := fw.Write(photo); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return t.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (t *TelegramOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, URL, message string, alert template.Alert) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[t.options.AlertExpression]
	if !ok {
		err := errors.New("No alert expression")
		return err
	}

	caption := alert.Labels["alertname"]
	unit := alert.Labels["unit"]

	var minutes *int

	if m, err := strconv.Atoi(alert.Labels["minutes"]); err == nil {
		minutes = &m
	}

	expr, err := metricsql.Parse(query)
	if err != nil {
		return err
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
		return t.sendMessage(span.GetContext(), URL, messageQuery)
	}

	photo, fileName, err := t.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		t.sendErrorMessage(span.GetContext(), URL, messageQuery, err)
		return nil
	}

	return t.sendPhoto(span.GetContext(), t.getSendPhotoURL(URL), messageQuery, fileName, photo)
}

func (t *TelegramOutput) Send(event *common.Event) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		if t.client == nil || t.message == nil {
			t.logger.Debug("No client or message")
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

		URLs := t.options.URL
		if t.selector != nil {
			b, err := t.selector.Execute(jsonObject)
			if err != nil {
				t.logger.SpanDebug(span, err)
			} else {
				URLs = b.String()
			}
		}

		if utils.IsEmpty(URLs) {
			t.logger.SpanError(span, "Telegram URLs are not found")
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

		arr := strings.Split(URLs, "\n")
		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if utils.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "AlertmanagerEvent":
				if err := t.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert)); err != nil {
					t.sendErrorMessage(span.GetContext(), URL, message, err)
				}
			default:
				t.sendMessage(span.GetContext(), URL, message)
			}
		}
	}()
}

func NewTelegramOutput(wg *sync.WaitGroup,
	options TelegramOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability) *TelegramOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) {
		logger.Debug("Telegram URL is not defined. Skipped")
		return nil
	}

	return &TelegramOutput{
		wg:       wg,
		client:   sreCommon.MakeHttpClient(options.Timeout),
		message:  render.NewTextTemplate("telegram-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("telegram-selector", options.URLSelector, templateOptions, options, logger),
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all telegram requests", []string{"chat_id"}, "telegram", "output"),
	}
}
