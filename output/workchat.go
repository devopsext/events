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

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
)

var workchatOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_workchat_output_count",
	Help: "Count of all workchat outputs",
}, []string{})

type WorkchatOutputOptions struct {
	MessageTemplate  string
	SelectorTemplate string
	URL              string
	Timeout          int
	AlertExpression  string
	NotificationType string
}

type WorkchatOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.Grafana
	options  WorkchatOutputOptions
}

func (w *WorkchatOutput) post(URL, contentType string, body bytes.Buffer, message string) (response interface{}, err error) {

	log.Debug("Post to Workchat (%s) => %s", URL, message)
	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	//workchatOutputCount.WithLabelValues(t.getBotID(URL)).Inc()

	log.Debug("Response from Workchat => %s", string(b))

	var object interface{}

	if err := json.Unmarshal(b, &object); err != nil {
		return nil, err
	}

	return object, nil
}

func (w *WorkchatOutput) sendMessage(URL, message string) error {

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	defer func() {
		if err := mw.Close(); err != nil {
			log.Warn("Failed to close writer")
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
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	return writer.CreatePart(h)
}

func (w *WorkchatOutput) sendPhoto(URL, message, fileName string, photo []byte) error {

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	defer func() {
		if err := mw.Close(); err != nil {
			log.Warn("Failed to close writer")
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

	response, err := w.post(URL, mw.FormDataContentType(), body, message)
	if err != nil {
		return err
	}

	if response != nil {
		return nil
	} else {
		return w.sendMessage(URL, message)
	}
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
		return errors.New("No alert expression")
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
		log.Error(err)
		w.sendErrorMessage(URL, messageQuery, err)
		return nil
	}

	return w.sendPhoto(URL, messageQuery, fileName, photo)
}

func (w *WorkchatOutput) Send(event *common.Event) {

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		if w.client == nil || w.message == nil {
			log.Error(errors.New("No client or message"))
			return
		}

		if event == nil {
			log.Error(errors.New("Event is empty"))
			return
		}

		if event.Data == nil {
			log.Error(errors.New("Event data is empty"))
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			log.Error(err)
			return
		}

		URLs := w.options.URL
		if w.selector != nil {

			b, err := w.selector.Execute(jsonObject)
			if err != nil {
				log.Error(err)
			} else {
				URLs = b.String()
			}
		}

		if common.IsEmpty(URLs) {
			log.Error(errors.New("Workchat URLs are not found"))
			return
		}

		b, err := w.message.Execute(jsonObject)
		if err != nil {
			log.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			log.Debug("Workchat message is empty")
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
				w.sendMessage(URL, message)
			case "AlertmanagerEvent":

				if err := w.sendAlertmanagerImage(URL, message, event.Data.(template.Alert)); err != nil {
					log.Error(err)
					w.sendErrorMessage(URL, message, err)
				}
			}
		}
	}()
}

func NewWorkchatOutput(wg *sync.WaitGroup,
	options WorkchatOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaOptions render.GrafanaOptions) *WorkchatOutput {

	if common.IsEmpty(options.URL) {
		log.Debug("Workchat URL is not defined. Skipped")
		return nil
	}

	return &WorkchatOutput{
		wg:       wg,
		client:   common.MakeHttpClient(options.Timeout),
		message:  render.NewTextTemplate("workchat-message", options.MessageTemplate, templateOptions, options),
		selector: render.NewTextTemplate("workchat-selector", options.SelectorTemplate, templateOptions, options),
		grafana:  render.NewGrafana(grafanaOptions),
		options:  options,
	}
}

func init() {
	prometheus.Register(workchatOutputCount)
}
