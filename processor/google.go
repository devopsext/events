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

type GoogleProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type GoogleResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

type GoogleMetric struct {
	Type        string            `json:"type"`
	DisplayName string            `json:"displayName"`
	Labels      map[string]string `json:"labels"`
}

type GoogleMetadata struct {
	SystemLabels map[string]string `json:"system_labels"`
	UserLabels   map[string]string `json:"user_labels"`
}

type GoogleConditionThreshold struct {
	Filter         string      `json:"filter"`
	Comparison     string      `json:"comparison"`
	ThresholdValue float32     `json:"thresholdValue"`
	Duration       string      `json:"duration"`
	Trigger        interface{} `json:"trigger"`
}

type GoogleCondition struct {
	Name               string                    `json:"name"`
	DisplayName        string                    `json:"displayName"`
	ConditionThreshold *GoogleConditionThreshold `json:"conditionThreshold"`
}

type GoogleIncident struct {
	IncidentID              string            `json:"incident_id"`
	ScopingProjectID        string            `json:"scoping_project_id"`
	ScopingProjectNumber    string            `json:"scoping_project_number"`
	URL                     string            `json:"url"`
	StartedAt               int64             `json:"started_at"`
	EndedAt                 int64             `json:"ended_at,omitempty"`
	State                   string            `json:"state"`
	Summary                 string            `json:"summary"`
	ApigeeURL               string            `json:"apigee_url"`
	ObservedValue           string            `json:"observed_value"`
	Resource                *GoogleResource   `json:"resource"`
	ResourceTypeDisplayName string            `json:"resource_type_display_name"`
	ResourceID              string            `json:"resource_id"`
	ResourceDisplayName     string            `json:"resource_display_name"`
	ResourceName            string            `json:"resource_name"`
	Metric                  *GoogleMetric     `json:"metric"`
	Metadata                *GoogleMetadata   `json:"metadata"`
	PolicyName              string            `json:"policy_name"`
	PolicyUserLabels        map[string]string `json:"policy_user_labels"`
	Documentation           string            `json:"documentation"`
	Condition               *GoogleCondition  `json:"condition"`
	ConditionName           string            `json:"condition_name"`
	ThresholdValue          string            `json:"threshold_value"`
}

type GoogleRequest struct {
	Version  string          `json:"version"`
	Incident *GoogleIncident `json:"incident"`
}

type GoogleResponse struct {
	Message string
}

func GoogleProcessorType() string {
	return "Google"
}

func (p *GoogleProcessor) EventType() string {
	return common.AsEventType(GoogleProcessorType())
}

func (p *GoogleProcessor) send(channel string, o interface{}, t *time.Time) {

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

func (p *GoogleProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("google", "requests", "Count of all google processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *GoogleProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("google", "requests", "Count of all google processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("google", "errors", "Count of all google processor errors", labels, "processor")

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

	var request GoogleRequest
	if err := json.Unmarshal(body, &request); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	if request.Incident.StartedAt > 0 {
		t := time.UnixMilli(request.Incident.StartedAt)
		p.send(channel, request, &t)
	} else {
		p.send(channel, request, nil)
	}

	response := &GoogleResponse{
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

func NewGoogleProcessor(outputs *common.Outputs, observability *common.Observability) *GoogleProcessor {

	return &GoogleProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
