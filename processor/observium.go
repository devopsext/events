package processor

import (
	"encoding/json"
	errPkg "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type ObserviumEventProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type ObserviumRequest struct {
	Title          string `json:"TITLE"`
	AlertState     string `json:"ALERT_STATE"`
	AlertURL       string `json:"ALERT_URL"`
	AlertUnixTime  int64  `json:"ALERT_UNIXTIME"`
	DeviceHostname string `json:"DEVICE_HOSTNAME"`
	DeviceLocation string `json:"DEVICE_LOCATION"`
	Metrics        string `json:"METRICS"`
}

type ObserviumResponse struct {
	Message string
}

func ObserviumEventProcessorType() string {
	return "Observium"
}

func (p *ObserviumEventProcessor) EventType() string {
	return common.AsEventType(ObserviumEventProcessorType())
}

func (p *ObserviumEventProcessor) send(channel string, o interface{}, t *time.Time) {

	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    o,
	}
	if t != nil && (*t).UnixNano() > 0 {
		e.SetTime((*t).UTC())
	} else {
		e.SetTime(time.Now().UTC())
	}
	e.SetLogger(p.logger)
	p.outputs.Send(e)
}

func (p *ObserviumEventProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("observium", "requests", "Count of all observium processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *ObserviumEventProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("observium", "requests", "Count of all observium processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("observium", "errors", "Count of all observium processor errors", labels, "processor")

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

	var observiumEvent ObserviumRequest
	if err := json.Unmarshal(body, &observiumEvent); err != nil {
		errors.Inc()
		p.logger.Error("Error decoding incoming message: %s", body)
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	t := time.Unix(observiumEvent.AlertUnixTime, 0)

	p.send(channel, observiumEvent, &t)

	response := &ObserviumResponse{
		Message: "OK",
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
	return nil
}

func NewObserviumEventProcessor(outputs *common.Outputs, observability *common.Observability) *ObserviumEventProcessor {

	return &ObserviumEventProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
	}
}
