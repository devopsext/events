package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type K8sEventProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type K8sEventData struct {
	Type     string      `json:"type"`
	Source   string      `json:"source"`
	Reason   string      `json:"reason"`
	Location string      `json:"location"`
	Message  string      `json:"message"`
	Object   interface{} `json:"object,omitempty"`
}

func (p *K8sEventProcessor) sendEE(span sreCommon.TracerSpan, channel string, e *common.EnhancedEvent) error {

	ce := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data: K8sEventData{
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

func (p *K8sEventProcessor) processEnhancedEvent(w http.ResponseWriter, span sreCommon.TracerSpan, channel string, e *common.EnhancedEvent) error {

	if err := p.sendEE(span, channel, e); err != nil {

	}
	if _, err := w.Write([]byte("OK")); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p K8sEventProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {
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

	var e *common.EnhancedEvent
	if err := json.Unmarshal(body, &e); err == nil {
		return p.processEnhancedEvent(w, span, channel, e)
	}
	errorString := fmt.Sprintf("Could not parse body as EnhancedEvent: %s", body)
	p.errors.Inc(channel)
	err := errors.New(errorString)
	p.logger.SpanError(span, errorString)
	http.Error(w, fmt.Sprint(errorString), http.StatusInternalServerError)
	return err
}

func K8sEventProcessorType() string {
	return "K8sE"
}

func (p K8sEventProcessor) EventType() string {
	return common.AsEventType(K8sEventProcessorType())
}

func (p K8sEventProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	return nil
}

func NewK8sEventProcessor(outputs *common.Outputs, observability *common.Observability) *K8sEventProcessor {
	return &K8sEventProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		//	counter: observability.Metrics().Counter("requests", "Count of all k8s processor requests", []string{"user", "operation", "channel", "namespace", "kind"}, "k8s", "processor"),
		requests: observability.Metrics().Counter("requests", "Count of all k8s processor requests", []string{"channel"}, "k8s", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all k8s processor errors", []string{"channel"}, "k8s", "processor"),
	}
}
