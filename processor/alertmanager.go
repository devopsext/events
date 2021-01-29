package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/devopsext/events/common"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
)

var AlertmanagerProcessorRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_alertmanager_processor_requests",
	Help: "Count of all alertmanager processor requests",
}, []string{})

type AlertmanagerProcessor struct {
	outputs *common.Outputs
}

type AlertmanagerResponse struct {
	Message string
}

func (p *AlertmanagerProcessor) prepareStatus(status string) string {

	return strings.Title(strings.ToLower(status))
}

func (p *AlertmanagerProcessor) processData(channel string, data *template.Data) {

	for _, alert := range data.Alerts {

		p.outputs.Send(&common.Event{
			Channel: channel,
			Type:    "AlertmanagerEvent",
			Data:    alert,
		})
	}
}

func (p *AlertmanagerProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	var body []byte

	if r.Body != nil {

		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {

		log.Error("Empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	log.Debug("Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {

		log.Error("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return
	}

	var response *AlertmanagerResponse

	data := template.Data{}
	if err := json.Unmarshal(body, &data); err != nil {

		log.Error("Can't decode body: %v", err)

		response = &AlertmanagerResponse{
			Message: err.Error(),
		}
	} else {

		channel := strings.TrimLeft(r.URL.Path, "/")
		p.processData(channel, &data)

		response = &AlertmanagerResponse{
			Message: "OK",
		}
	}

	resp, err := json.Marshal(response)
	if err != nil {

		log.Error("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {

		log.Error("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

}

func NewAlertmanagerProcessor(outputs *common.Outputs) *AlertmanagerProcessor {
	return &AlertmanagerProcessor{outputs: outputs}
}

func init() {
	prometheus.Register(AlertmanagerProcessorRequests)
}