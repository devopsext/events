package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type ObserviumEventProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
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

func (p *ObserviumEventProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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
	if span != nil {
		e.SetSpanContext(span.GetContext())
		e.SetLogger(p.logger)
	}
	p.outputs.Send(e)
}

func (p *ObserviumEventProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *ObserviumEventProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

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

	var observiumEvent ObserviumRequest
	if err := json.Unmarshal(body, &observiumEvent); err != nil {
		p.errors.Inc(channel)
		p.logger.Error("Error decoding incoming message: %s", body)
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	t := time.Unix(observiumEvent.AlertUnixTime, 0)

	p.send(span, channel, observiumEvent, &t)

	response := &ObserviumResponse{
		Message: "OK",
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
	return nil
}

func NewObserviumEventProcessor(outputs *common.Outputs, observability *common.Observability) *ObserviumEventProcessor {

	return &ObserviumEventProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all Observium processor requests", []string{"channel"}, "observium", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all Observium processor errors", []string{"channel"}, "observium", "processor"),
	}
}
