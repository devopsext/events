package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/prometheus/alertmanager/template"
)

type CustomJsonProcessor struct {
	outputs *common.Outputs
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
}

type CustomJsonResponse struct {
	Message string
}

func (p *CustomJsonProcessor) Type() string {
	return "CustomJsonEvent"
}

func (p *CustomJsonProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {

		p.logger.SpanError(span, "Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return
	}

	var response *CustomJsonResponse

	data := template.Data{}
	if err := json.Unmarshal(body, &data); err != nil {

		p.logger.SpanError(span, "Can't decode body: %v", err)

		response = &CustomJsonResponse{
			Message: err.Error(),
		}
	} else {

		//channel := strings.TrimLeft(r.URL.Path, "/")
		//p.processData(span, channel, &data)

		response = &CustomJsonResponse{
			Message: "OK",
		}
	}

	resp, err := json.Marshal(response)
	if err != nil {
		p.logger.SpanError(span, "Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func NewCustomJsonProcessor(outputs *common.Outputs, logger sreCommon.Logger, tracer sreCommon.Tracer, meter sreCommon.Meter) *CustomJsonProcessor {

	return &CustomJsonProcessor{
		outputs: outputs,
		logger:  logger,
		tracer:  tracer,
	}
}
