package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"io"
	"k8s.io/api/core/v1"
	"net/http"
	"strings"
	"time"
)

type KubeProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type KubeData struct {
	Type     string      `json:"type"`
	Source   string      `json:"source"`
	Reason   string      `json:"reason"`
	Location string      `json:"location"`
	Message  string      `json:"message"`
	Object   interface{} `json:"object,omitempty"`
}

type EnhancedObjectReference struct {
	v1.ObjectReference `json:",inline"`
	Labels             map[string]string `json:"labels,omitempty"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}

// EnhancedEvent Original file https://github.com/opsgenie/kubernetes-event-exporter/blob/master/pkg/kube/event.go
type EnhancedEvent struct {
	v1.Event       `json:",inline"`
	InvolvedObject EnhancedObjectReference `json:"involvedObject"`
}

func (p *KubeProcessor) send(span sreCommon.TracerSpan, channel string, e *EnhancedEvent) error {
	ce := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data: KubeData{
			Reason:   e.Reason,
			Message:  e.Message,
			Type:     e.Type,
			Location: fmt.Sprintf("%s/%s", e.Namespace, e.Name),
			Source:   fmt.Sprintf("%s/%s", e.Source.Host, e.Source.Component),
			Object:   e.InvolvedObject,
		},
	}
	ce.SetTime(time.Now().UTC())
	if span != nil {
		ce.SetSpanContext(span.GetContext())
		ce.SetLogger(p.logger)
	}
	p.outputs.Send(ce)
	return nil
}

func (p *KubeProcessor) processEvent(
	w http.ResponseWriter,
	span sreCommon.TracerSpan,
	channel string,
	e *EnhancedEvent,
) error {
	if err := p.send(span, channel, e); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, "Can't send event: %v", err)
		http.Error(w, fmt.Sprintf("couldn't send event: %v", err), http.StatusInternalServerError)
		return err
	}
	if _, err := w.Write([]byte("OK")); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p *KubeProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {
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

	var e *EnhancedEvent
	if err := json.Unmarshal(body, &e); err == nil {
		return p.processEvent(w, span, channel, e)
	}
	errorString := fmt.Sprintf("Could not parse body as EnhancedEvent: %s", body)
	p.errors.Inc(channel)
	err := errors.New(errorString)
	p.logger.SpanError(span, errorString)
	http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
	return err
}

func KubeProcessorType() string {
	return "Kube"
}

func (p *KubeProcessor) EventType() string {
	return common.AsEventType(KubeProcessorType())
}

func (p *KubeProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	return nil
}

// DeDot replaces all dots in the labels and annotations with underscores. This is required for example in the
// elasticsearch sink. The dynamic mapping generation interprets dots in JSON keys as path in an object.
// For reference see this logstash filter: https://www.elastic.co/guide/en/logstash/current/plugins-filters-de_dot.html
func (e EnhancedEvent) DeDot() EnhancedEvent {
	c := e
	c.Labels = common.DeDotMap(e.Labels)
	c.Annotations = common.DeDotMap(e.Annotations)
	c.InvolvedObject.Labels = common.DeDotMap(e.InvolvedObject.Labels)
	c.InvolvedObject.Annotations = common.DeDotMap(e.InvolvedObject.Annotations)
	return c
}

// ToJSON does not return an error because we are %99 confident it is JSON serializable.
func (e EnhancedEvent) ToJSON() []byte {
	b, _ := json.Marshal(e)
	return b
}

func (e EnhancedEvent) GetTimestampMs() int64 {
	timestamp := e.FirstTimestamp.Time
	if timestamp.IsZero() {
		timestamp = e.EventTime.Time
	}

	return timestamp.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func (e EnhancedEvent) GetTimestampISO8601() string {
	timestamp := e.FirstTimestamp.Time
	if timestamp.IsZero() {
		timestamp = e.EventTime.Time
	}

	layout := "2006-01-02T15:04:05.000Z"
	return timestamp.Format(layout)
}

func NewKubeProcessor(outputs *common.Outputs, observability *common.Observability) *KubeProcessor {
	return &KubeProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		requests: observability.Metrics().Counter(
			"requests",
			"Count of all kube processor requests",
			[]string{"channel"},
			"kube",
			"processor",
		),
		errors: observability.Metrics().Counter(
			"errors",
			"Count of all kube processor errors",
			[]string{"channel"},
			"kube",
			"processor",
		),
	}
}
