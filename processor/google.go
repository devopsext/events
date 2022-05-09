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

type GoogleProcessor struct {
	outputs *common.Outputs
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	counter sreCommon.Counter
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

func (p *GoogleProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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
	p.counter.Inc(e.Channel)
}

func (p *GoogleProcessor) HandleEvent(e *common.Event) {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return
	}

	p.outputs.Send(e)
	p.counter.Inc(e.Channel)
}

func (p *GoogleProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

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

	var request GoogleRequest
	if err := json.Unmarshal(body, &request); err != nil {
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return
	}

	channel := strings.TrimLeft(r.URL.Path, "/")
	if request.Incident.StartedAt > 0 {
		t := time.UnixMilli(request.Incident.StartedAt)
		p.send(span, channel, request, &t)
	} else {
		p.send(span, channel, request, nil)
	}

	response := &GoogleResponse{
		Message: "OK",
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

func NewGoogleProcessor(outputs *common.Outputs, observability *common.Observability) *GoogleProcessor {

	return &GoogleProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		counter: observability.Metrics().Counter("requests", "Count of all google processor requests", []string{"channel"}, "google", "processor"),
	}
}
