package processor

import (
	"encoding/json"
	errPkg "errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"github.com/prometheus/alertmanager/template"
)

type CustomJsonProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
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

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("customjson", "requests", "Count of all customjson processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("customjson", "errors", "Count of all customjson processor errors", labels, "processor")

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		errors.Inc()
		err := errPkg.New("empty body")
		p.logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p.logger.Debug("Body => %s", body)

	var response *CustomJsonResponse
	errorString := ""
	data := template.Data{}
	if err := json.Unmarshal(body, &data); err != nil {
		errorString = err.Error()
		p.logger.Error("Can't decode body: %v", err)
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
		errors.Inc()
		p.logger.Error("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	if _, err := w.Write(resp); err != nil {
		errors.Inc()
		p.logger.Error("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}

	if !utils.IsEmpty(errorString) {
		errors.Inc()
		err := errPkg.New(errorString)
		p.logger.Error(errorString)
		http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
		return err
	}
	return nil
}

func NewCustomJsonProcessor(outputs *common.Outputs, observability *common.Observability) *CustomJsonProcessor {

	return &CustomJsonProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
