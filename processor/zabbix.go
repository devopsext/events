package processor

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type ZabbixProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type ZabbixEvent struct {
	AlertURL           string `json:"AlertURL,omitempty"`
	Environment        string `json:"Environment,omitempty"`
	EventDate          string `json:"EventDate,omitempty"`
	EventID            string `json:"EventID,omitempty"`
	EventName          string `json:"EventName,omitempty"`
	EventNSeverity     string `json:"EventNSeverity,omitempty"`
	EventOpData        string `json:"EventOpData,omitempty"`
	Status             string `json:"Status,omitempty"`
	EventTags          string `json:"EventTags,omitempty"`
	EventTime          string `json:"EventTime,omitempty"`
	EventType          string `json:"EventType,omitempty"`
	HostName           string `json:"HostName,omitempty"`
	ItemID             string `json:"ItemID,omitempty"`
	ItemLastValue      string `json:"ItemLastValue,omitempty"`
	TriggerDescription string `json:"TriggerDescription,omitempty"`
	TriggerExpression  string `json:"TriggerExpression,omitempty"`
	TriggerName        string `json:"TriggerName,omitempty"`
}

func ZabbixProcessorType() string {
	return "Zabbix"
}

func (p *ZabbixProcessor) EventType() string {
	return common.AsEventType(ZabbixProcessorType())
}

func (p *ZabbixProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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

func (p *ZabbixProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc()
	p.outputs.Send(e)
	return nil
}

func (p *ZabbixProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	channel := strings.TrimLeft(r.URL.Path, "/")
	p.requests.Inc()

	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		p.errors.Inc()
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	var request ZabbixEvent

	err := json.Unmarshal(body, &request)
	if err != nil {
		p.errors.Inc()
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	EventDateTime, err := time.Parse(time.RFC3339Nano, strings.ReplaceAll(request.EventDate, ".", "-")+"T"+request.EventTime+"Z")
	if err != nil {
		p.send(span, channel, request, nil)
		return nil
	}
	p.send(span, channel, request, &EventDateTime)
	return nil
}

func NewZabbixProcessor(outputs *common.Outputs, observability *common.Observability) *ZabbixProcessor {

	return &ZabbixProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("zabbix", "requests", "Count of all zabbix processor requests", map[string]string{}, "processor"),
		errors:   observability.Metrics().Counter("zabbix", "errors", "Count of all zabbix processor errors", map[string]string{}, "processor"),
	}
}
