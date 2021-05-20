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

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
)

var telegramOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_telegram_output_count",
	Help: "Count of all telegram outputs",
}, []string{"telegram_output_bot"})

type TelegramOutputOptions struct {
	MessageTemplate     string
	SelectorTemplate    string
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
	grafana  *render.Grafana
	options  TelegramOutputOptions
	tracer   common.Tracer
}

// assume that url => https://api.telegram.org/bot508526210:sdsdfsdfsdf/sendMessage?chat_id=-324234234

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

func (t *TelegramOutput) post(spanCtx common.TracerSpanContext, URL, contentType string, body bytes.Buffer, message string) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	log.Debug("Post to Telegram (%s) => %s", URL, message)
	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		span.Error(err)
		return err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := t.client.Do(req)
	if err != nil {
		span.Error(err)
		return err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		span.Error(err)
		return err
	}

	telegramOutputCount.WithLabelValues(t.getBotID(URL)).Inc()

	log.Debug("Response from Telegram => %s", string(b))

	return nil
}

func (t *TelegramOutput) sendMessage(spanCtx common.TracerSpanContext, URL, message string) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			log.Warn("Failed to close writer")
		}
	}()

	if err := w.WriteField("text", message); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("parse_mode", "HTML"); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("disable_notification", t.options.DisableNotification); err != nil {
		span.Error(err)
		return err
	}

	if err := w.Close(); err != nil {
		span.Error(err)
		return err
	}

	return t.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (t *TelegramOutput) sendErrorMessage(spanCtx common.TracerSpanContext, URL, message string, err error) error {
	return t.sendMessage(spanCtx, URL, fmt.Sprintf("%s\n%s", message, err.Error()))
}

func (t *TelegramOutput) sendPhoto(spanCtx common.TracerSpanContext, URL, message, fileName string, photo []byte) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			log.Warn("Failed to close writer")
		}
	}()

	if err := w.WriteField("caption", message); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("parse_mode", "HTML"); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("disable_web_page_preview", "true"); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("disable_notification", t.options.DisableNotification); err != nil {
		span.Error(err)
		return err
	}

	fw, err := w.CreateFormFile("photo", fileName)
	if err != nil {
		span.Error(err)
		return err
	}

	if _, err := fw.Write(photo); err != nil {
		span.Error(err)
		return err
	}

	if err := w.Close(); err != nil {
		span.Error(err)
		return err
	}

	return t.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (t *TelegramOutput) sendAlertmanagerImage(spanCtx common.TracerSpanContext, URL, message string, alert template.Alert) error {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		span.Error(err)
		return err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[t.options.AlertExpression]
	if !ok {
		err := errors.New("No alert expression")
		span.Error(err)
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
		span.Error(err)
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
		log.Error(err)
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
			log.Error(errors.New("No client or message"))
			return
		}

		if event == nil {
			log.Error(errors.New("Event is empty"))
			return
		}

		span := t.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			err := errors.New("Event data is empty")
			log.Error(err)
			span.Error(err)
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			log.Error(err)
			span.Error(err)
			return
		}

		URLs := t.options.URL
		if t.selector != nil {

			b, err := t.selector.Execute(jsonObject)
			if err != nil {
				log.Error(err)
			} else {
				URLs = b.String()
			}
		}

		if common.IsEmpty(URLs) {
			err := errors.New("Telegram URLs are not found")
			log.Error(err)
			span.Error(err)
			return
		}

		b, err := t.message.Execute(jsonObject)
		if err != nil {
			log.Error(err)
			span.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			log.Debug("Telegram message is empty")
			return
		}

		arr := strings.Split(URLs, "\n")

		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if common.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "K8sEvent":
				t.sendMessage(span.GetContext(), URL, message)
			case "AlertmanagerEvent":

				if err := t.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert)); err != nil {
					log.Error(err)
					t.sendErrorMessage(span.GetContext(), URL, message, err)
				}
			}
		}
	}()
}

func NewTelegramOutput(wg *sync.WaitGroup,
	options TelegramOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaOptions render.GrafanaOptions,
	tracer common.Tracer) *TelegramOutput {

	if common.IsEmpty(options.URL) {
		log.Debug("Telegram URL is not defined. Skipped")
		return nil
	}

	return &TelegramOutput{
		wg:       wg,
		client:   common.MakeHttpClient(options.Timeout),
		message:  render.NewTextTemplate("telegram-message", options.MessageTemplate, templateOptions, options),
		selector: render.NewTextTemplate("telegram-selector", options.SelectorTemplate, templateOptions, options),
		grafana:  render.NewGrafana(grafanaOptions, tracer),
		options:  options,
		tracer:   tracer,
	}
}

func init() {
	prometheus.Register(telegramOutputCount)
}
