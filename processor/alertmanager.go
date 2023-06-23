package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"io"
	"net/http"
	"strings"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"github.com/prometheus/alertmanager/template"
)

type AlertmanagerProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type AlertmanagerResponse struct {
	Message string
}

func AlertmanagerProcessorType() string {
	return "Alertmanager"
}

func (p *AlertmanagerProcessor) EventType() string {
	return common.AsEventType(AlertmanagerProcessorType())
}

func (p *AlertmanagerProcessor) prepareStatus(status string) string {
	lower := cases.Lower(language.English)
	title := cases.Title(language.English)
	return title.String(lower.String(status))
}

func (p *AlertmanagerProcessor) send(span sreCommon.TracerSpan, channel string, data *template.Data) {

	for _, alert := range data.Alerts {

		e := &common.Event{
			Channel: channel,
			Type:    p.EventType(),
			Data:    alert,
		}
		e.SetTime(alert.StartsAt.UTC())
		if span != nil {
			e.SetSpanContext(span.GetContext())
			e.SetLogger(p.logger)
		}
		p.outputs.Send(e)
	}
}

func (p *AlertmanagerProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *AlertmanagerProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	channel := strings.TrimLeft(r.URL.Path, "/")
	p.requests.Inc(channel)

	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		p.errors.Inc(channel)
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	var response *AlertmanagerResponse
	errorString := ""
	data := template.Data{}
	if err := json.Unmarshal(body, &data); err != nil {
		p.logger.SpanError(span, "Can't decode body: %v", err)
		response = &AlertmanagerResponse{
			Message: err.Error(),
		}
		errorString = response.Message
	} else {
		p.send(span, channel, &data)
		response = &AlertmanagerResponse{
			Message: "OK",
		}
	}

	resp, err := json.Marshal(response)
	if err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, "Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	if _, err := w.Write(resp); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}

	if !utils.IsEmpty(errorString) {
		p.errors.Inc(channel)
		err := errors.New(errorString)
		p.logger.SpanError(span, errorString)
		http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
		return err
	}
	return nil
}

func NewAlertmanagerProcessor(outputs *common.Outputs, observability *common.Observability) *AlertmanagerProcessor {

	return &AlertmanagerProcessor{
		outputs:  outputs,
		tracer:   observability.Traces(),
		logger:   observability.Logs(),
		requests: observability.Metrics().Counter("requests", "Count of all alertmanager processor requests", []string{"channel"}, "alertmanager", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all alertmanager processor errors", []string{"channel"}, "alertmanager", "processor"),
	}
}
