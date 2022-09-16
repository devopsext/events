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

type ZabbixEvent struct {
	AlertURL           string
	Environment        string
	EventDate          string
	EventID            string
	EventName          string
	EventNSeverity     string
	EventOpData        string
	Status             string
	EventTags          string
	EventTime          string
	EventType          string
	HostName           string
	ItemID             string
	ItemLastValue      string
	TriggerDescription string
	TriggerExpression  string
	TriggerName        string
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

	var request ZabbixEvent

	paths := [][]string{
		[]string{"AlertURL"},
		[]string{"Environment"},
		[]string{"EventDate"},
		[]string{"EventID"},
		[]string{"EventName"},
		[]string{"EventNSeverity"},
		[]string{"EventOpData"},
		[]string{"Status"},
		[]string{"EventTags"},
		[]string{"EventTime"},
		[]string{"EventType"},
		[]string{"HostName"},
		[]string{"ItemID"},
		[]string{"ItemLastValue"},
		[]string{"TriggerDescription"},
		[]string{"TriggerExpression"},
		[]string{"TriggerName"},
	}
	var EventDate, EventTime string
	jsonparser.EachKey(body, func(idx int, value []byte, vt jsonparser.ValueType, err error) {
		u, _ := jsonparser.ParseString(value)
		v := string(u)
		switch idx {
		case 0:
			request.AlertURL = v
		case 1:
			request.Environment = v
		case 2:
			request.EventDate = v
		case 3:
			request.EventID = v
		case 4:
			request.EventName = v
		case 5:
			request.EventNSeverity = v
		case 6:
			request.EventOpData = v
		case 7:
			request.Status = v
		case 8:
			request.EventTags = v
		case 9:
			request.EventTime = v
		case 10:
			request.EventType = v
		case 11:
			request.HostName = v
		case 12:
			request.ItemID = v
		case 13:
			request.ItemLastValue = v
		case 14:
			request.TriggerDescription = v
		case 15:
			request.TriggerExpression = v
		case 16:
			request.TriggerName = v
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
