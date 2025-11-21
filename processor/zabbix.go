package processor

import (
	"encoding/json"
	errPkg "errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type ZabbixProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
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

func (p *ZabbixProcessor) send(channel string, o interface{}, t *time.Time) {

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

func (p *ZabbixProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("zabbix", "requests", "Count of all zabbix processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *ZabbixProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("zabbix", "requests", "Count of all zabbix processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("zabbix", "errors", "Count of all zabbix processor errors", labels, "processor")

	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
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

	var request ZabbixEvent

	err := json.Unmarshal(body, &request)
	if err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	EventDateTime, err := time.Parse(time.RFC3339Nano, strings.ReplaceAll(request.EventDate, ".", "-")+"T"+request.EventTime+"Z")
	if err != nil {
		p.send(channel, request, nil)
		return nil
	}
	p.send(channel, request, &EventDateTime)
	return nil
}

func NewZabbixProcessor(outputs *common.Outputs, observability *common.Observability) *ZabbixProcessor {

	return &ZabbixProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
	}
}
