package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"github.com/prometheus/alertmanager/template"
)

type CustomJsonProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type CustomJsonResponse struct {
	Message string
}

func CustomJsonProcessorType() string {
	return "CustomJson"
}

func (p *CustomJsonProcessor) EventType() string {
	return common.AsEventType(CustomJsonProcessorType())
}

func (p *CustomJsonProcessor) HandleEvent(e *common.Event) error {
	return nil
}

func (p *CustomJsonProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	channel := strings.TrimLeft(r.URL.Path, "/")
	p.requests.Inc(channel)

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
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

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		p.errors.Inc(channel)
		err := fmt.Errorf("Content-Type=%s, expect application/json", contentType)
		p.logger.SpanError(span, err)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return err
	}

	var response *CustomJsonResponse
	errorString := ""
	data := template.Data{}
	if err := json.Unmarshal(body, &data); err != nil {
		errorString = errorString
		p.logger.SpanError(span, "Can't decode body: %v", err)
		response = &CustomJsonResponse{
			Message: errorString,
		}
	} else {

		response = &CustomJsonResponse{
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

func NewCustomJsonProcessor(outputs *common.Outputs, observability *common.Observability) *CustomJsonProcessor {

	return &CustomJsonProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all customjson processor requests", []string{"channel"}, "customjson", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all customjson processor errors", []string{"channel"}, "customjson", "processor"),
	}
}
