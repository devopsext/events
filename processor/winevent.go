package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type WinEventRequest struct {
	Message      string      `json:"Message"`
	ProcessId    int         `json:"ProcessId"`
	ThreadId     int         `json:"ThreadId"`
	Time         string      `json:"TimeCreated"`
	Host         string      `json:"MachineName"`
	MessageLevel string      `json:"LevelDisplayName"`
	EventId      int         `json:"Id"`
	Properties   interface{} `json:"Properties,omitempty"`
}

type WinEventResponse struct {
	Message string
}

type WinEventProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

func WinEventProcessorType() string {
	return "WinEvent"
}

func (p *WinEventProcessor) EventType() string {
	return common.AsEventType(WinEventProcessorType())
}

func (p *WinEventProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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

func (p *WinEventProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *WinEventProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

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

	var WinEvent WinEventRequest
	if err := json.Unmarshal(body, &WinEvent); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindStringSubmatch("/Date(1653782726738)/")[1]
	intTime, err := strconv.Atoi(match)
	if err == nil {
		return err
	}
	t := time.UnixMilli(int64(intTime))
	p.send(span, channel, WinEvent, &t)

	response := &WinEventResponse{
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

func NewWinEventProcessor(outputs *common.Outputs, observability *common.Observability) *WinEventProcessor {

	return &WinEventProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all WinEvent processor requests", []string{"channel"}, "WinEvent", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all WinEvent processor errors", []string{"channel"}, "WinEvent", "processor"),
	}
}
