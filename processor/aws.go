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

type AWSProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type AWSRequest struct {
	Version    string      `json:"version"`
	Source     string      `json:"source"`
	Account    string      `json:"account"`
	Time       time.Time   `json:"time"`
	Region     string      `json:"region"`
	DetailType string      `json:"detail-type,omitempty"`
	Detail     interface{} `json:"detail,omitempty"`
}

type AWSResponse struct {
	Message string
}

func AWSProcessorType() string {
	return "AWS"
}

func (p *AWSProcessor) EventType() string {
	return common.AsEventType(AWSProcessorType())
}

func (p *AWSProcessor) send(channel string, o interface{}, t *time.Time) {

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

func (p *AWSProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("aws", "requests", "Count of all aws processor requests", labels, "processor")
	requests.Inc()
	p.outputs.Send(e)
	return nil
}

func (p *AWSProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("aws", "requests", "Count of all aws processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("aws", "errors", "Count of all aws processor errors", labels, "processor")

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

	var request AWSRequest
	if err := json.Unmarshal(body, &request); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	if request.Time.UnixMilli() > 0 {
		t := request.Time
		p.send(channel, request, &t)
	} else {
		p.send(channel, request, nil)
	}

	response := &AWSResponse{
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

func NewAWSProcessor(outputs *common.Outputs, observability *common.Observability) *AWSProcessor {

	return &AWSProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
