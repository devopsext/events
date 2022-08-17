package processor

import (
	"errors"
	"github.com/buger/jsonparser"
	"io/ioutil"
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

type zabbixEvent struct {
	EventType          string
	EventNSeverity     string
	Status             string
	EventID            string
	HostName           string
	ItemID             string
	ItemLastValue      string
	AlertURL           string
	TriggerDescription string
	EventTags          string
	Scope              string
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
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *ZabbixProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

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

	var request zabbixEvent

	paths := [][]string{
		[]string{"EventType"},
		[]string{"EventNSeverity"},
		[]string{"Status"},
		[]string{"EventID"},
		[]string{"EventTime"},
		[]string{"EventDate"},
		[]string{"HostName"},
		[]string{"ItemID"},
		[]string{"ItemLastValue"},
		[]string{"AlertURL"},
		[]string{"TriggerDescription"},
		[]string{"EventTags"},
		[]string{"Scope"},
	}
	var EventDate, EventTime string
	jsonparser.EachKey(body, func(idx int, value []byte, vt jsonparser.ValueType, err error) {
		switch idx {
		case 0:
			v, _ := jsonparser.ParseString(value)
			request.EventType = string(v)
		case 1:
			v, _ := jsonparser.ParseString(value)
			request.EventNSeverity = string(v)
		case 2:
			v, _ := jsonparser.ParseString(value)
			request.Status = string(v)
		case 3:
			v, _ := jsonparser.ParseString(value)
			request.EventID = string(v)
		case 4:
			v, _ := jsonparser.ParseString(value)
			EventTime = string(v)
		case 5:
			v, _ := jsonparser.ParseString(value)
			EventDate = string(v)
		case 6:
			v, _ := jsonparser.ParseString(value)
			request.HostName = string(v)
		case 7:
			v, _ := jsonparser.ParseString(value)
			request.ItemID = string(v)
		case 8:
			v, _ := jsonparser.ParseString(value)
			request.ItemLastValue = string(v)
		case 9:
			v, _ := jsonparser.ParseString(value)
			request.AlertURL = string(v)
		case 10:
			v, _ := jsonparser.ParseString(value)
			request.TriggerDescription = string(v)
		case 11:
			v, _ := jsonparser.ParseString(value)
			request.EventTags = string(v)
		case 12:
			v, _ := jsonparser.ParseString(value)
			request.Scope = string(v)
		}
	}, paths...)

	EventDateTime, err := time.Parse(time.RFC3339Nano, strings.ReplaceAll(EventDate, ".", "-")+"T"+EventTime+"Z")
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
		requests: observability.Metrics().Counter("requests", "Count of all google processor requests", []string{"channel"}, "zabbix", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all google processor errors", []string{"channel"}, "zabbix", "processor"),
	}
}
