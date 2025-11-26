package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/devopsext/utils"
	"github.com/prometheus/alertmanager/template"
)

type WorkchatOutputOptions struct {
	Message          string
	URLSelector      string
	URL              string
	Timeout          int
	AlertExpression  string
	NotificationType string
}

type WorkchatOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	message  *toolsRender.TextTemplate
	selector *toolsRender.TextTemplate
	grafana  *render.GrafanaRender
	options  WorkchatOutputOptions
	logger   sreCommon.Logger
	meter    sreCommon.Meter
}

func (w *WorkchatOutput) Name() string {
	return "Workchat"
}

// assume that url is => https://graph.workplace.com/v9.0/me/messages?access_token=%s&recipient=%s
func (w *WorkchatOutput) getThread(URL string) string {

	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}

	recipientJson := u.Query().Get("recipient")
	if utils.IsEmpty(recipientJson) {
		return ""
	}

	var object map[string]interface{}

	if err := json.Unmarshal([]byte(recipientJson), &object); err != nil {
		return ""
	}

	threadKey := object["thread_key"]
	if threadKey == nil {
		return ""
	}

	return threadKey.(string)
}

func (w *WorkchatOutput) post(URL, contentType string, body bytes.Buffer, message string) (response interface{}, err error) {

	w.logger.Debug("Post to Workchat (%s) => %s", URL, message)
	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		w.logger.Error(err)
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error(err)
		return nil, err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.logger.Error(err)
		return nil, err
	}

	w.logger.Debug("Response from Workchat => %s", string(b))

	var object interface{}

	if err := json.Unmarshal(b, &object); err != nil {
		w.logger.Error(err)
		return nil, err
	}

	return object, nil
}

func (w *WorkchatOutput) sendMessage(URL, message string) error {

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	defer func() {
		if err := mw.Close(); err != nil {
			w.logger.Warn("Failed to close writer")
		}
	}()

	m := fmt.Sprintf("{\"text\":\"%s\"}", strings.ReplaceAll(message, "\n", "\\n"))
	if err := mw.WriteField("message", m); err != nil {
		return err
	}

	if err := mw.WriteField("notification_type", w.options.NotificationType); err != nil {
		return err
	}

	if err := mw.Close(); err != nil {
		return err
	}

	_, err := w.post(URL, mw.FormDataContentType(), body, message)
	return err
}

func (w *WorkchatOutput) sendErrorMessage(URL, message string, err error) error {
	return w.sendMessage(URL, fmt.Sprintf("%s\n%s", message, err.Error()))
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (w *WorkchatOutput) createFormFile(writer *multipart.Writer, fieldname, filename, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	return writer.CreatePart(h)
}

func (w *WorkchatOutput) sendPhoto(URL, message, fileName string, photo []byte) error {

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	defer func() {
		if err := mw.Close(); err != nil {
			w.logger.Warn("Failed to close writer")
		}
	}()

	m := "{\"attachment\":{\"type\":\"image\",\"payload\":{\"is_reusable\":true}}}"
	if err := mw.WriteField("message", m); err != nil {
		return err
	}

	fw, err := w.createFormFile(mw, "filedata", fileName, "image/png")
	if err != nil {
		return err
	}

	if _, err := fw.Write(photo); err != nil {
		return err
	}

	if err := mw.Close(); err != nil {
		return err
	}

	_, err = w.post(URL, mw.FormDataContentType(), body, message)
	if err != nil {
		return err
	}

	return w.sendMessage(URL, message)
}

func (w *WorkchatOutput) sendAlertmanagerImage(URL, message string, alert template.Alert) error {

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[w.options.AlertExpression]
	if !ok {
		err := errors.New("no alert expression")
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

	messageQuery := fmt.Sprintf("%s\n_%s_", message, query)

	if w.grafana == nil {
		return w.sendMessage(URL, messageQuery)
	}

	photo, fileName, err := w.grafana.GenerateDashboard(caption, metric, operator, value, minutes, unit)
	if err != nil {
		w.sendErrorMessage(URL, messageQuery, err)
		return nil
	}

	return w.sendPhoto(URL, messageQuery, fileName, photo)
}

func (w *WorkchatOutput) Send(event *common.Event) {

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		if event == nil {
			w.logger.Debug("Event is empty")
			return
		}

		if event.Data == nil {
			w.logger.Error("Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			w.logger.Error(err)
			return
		}

		URLs := w.options.URL
		if w.selector != nil {

			b, err := w.selector.RenderObject(jsonObject)
			if err != nil {
				w.logger.Debug(err)
			} else {
				URLs = string(b)
			}
		}

		if utils.IsEmpty(URLs) {
			w.logger.Error("Workchat URLs are not found")
			return
		}

		b, err := w.message.RenderObject(jsonObject)
		if err != nil {
			w.logger.Error(err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			w.logger.Debug("Workchat message is empty")
			return
		}

		w.logger.Debug("Workchat message => %s", message)

		arr := strings.Split(URLs, "\n")

		labels := make(map[string]string)
		labels["event_channel"] = event.Channel
		labels["event_type"] = event.Type
		labels["output"] = w.Name()

		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if utils.IsEmpty(URL) {
				continue
			}

			labels["workchat_url"] = URL
			requests := w.meter.Counter("workchat", "requests", "Count of all workchar requests", labels, "output")

			// thread := w.getThread(URL)
			requests.Inc()

			errors := w.meter.Counter("workchat", "errors", "Count of all workchar requests", labels, "output")

			switch event.Type {
			case "AlertmanagerEvent":
				if err := w.sendAlertmanagerImage(URL, message, event.Data.(template.Alert)); err != nil {
					errors.Inc()
					w.sendErrorMessage(URL, message, err)
				}
			default:
				err := w.sendMessage(URL, message)
				if err != nil {
					errors.Inc()
				}
			}
		}
	}()
}

func NewWorkchatOutput(wg *sync.WaitGroup,
	options WorkchatOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability) *WorkchatOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) {
		logger.Debug("Workchat URL is not defined. Skipped")
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "workchat-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	selectorOpts := toolsRender.TemplateOptions{
		Name:       "workchat-selector",
		Content:    common.Content(options.URLSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	selector, err := toolsRender.NewTextTemplate(selectorOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &WorkchatOutput{
		wg:       wg,
		client:   utils.NewHttpInsecureClient(options.Timeout),
		message:  message,
		selector: selector,
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		logger:   logger,
		meter:    observability.Metrics(),
	}
}
