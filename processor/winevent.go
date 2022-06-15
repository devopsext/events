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

type WinEventPubsubRequest struct {
	Time     string                   `json:"time"`
	Channel  string                   `json:"channel"`
	Original *WinEventOriginalRequest `json:"data"`
}
type WinEventOriginalRequest struct {
	Events []WinEvent `json:"metrics"`
}
type WinEventTags struct {
	Message      string `json:"Message"`
	MessageLevel string `json:"LevelText"`
	Keywords     string `json:"Keywords"`
	EventId      string `json:"EventRecordID"`
	Provider     string `json:"provider"`
	City         string `json:"city"`
	Country      string `json:"country,omitempty"`
	MT           string `json:"mt"`
	Host         string `json:"host"`
}
type WinEvent struct {
	Name      string        `json:"name"`
	Tags      *WinEventTags `json:"tags"`
	Timestamp int64         `json:"timestamp"`
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
	return "Win"
}

func (p *WinEventProcessor) EventType() string {
	return common.AsEventType(WinEventProcessorType())
}

func (p *WinEventProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	if js, err := e.JsonBytes(); err == nil {
		var PubSubEvents WinEventPubsubRequest
		if err := json.Unmarshal(js, &PubSubEvents); err != nil {
			p.errors.Inc(e.Channel)
			p.logger.Error("Failed while unmarshalling: %s", err)
			return err
		}
		p.logger.Debug("After repeatitive unmarshall we got: %s", PubSubEvents)
		for _, event := range PubSubEvents.Original.Events {
			newEvent := &common.Event{
				Channel: e.Channel,
				Type:    e.Type,
				Data:    event,
			}
			t := time.UnixMilli(event.Timestamp)
			e.SetTime((t).UTC())
			p.requests.Inc(e.Channel)
			p.outputs.Send(newEvent)
		}
	}
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
	// body_new := body
	// var pubsubevent common.Event
	// if err := json.Unmarshal(body_new, &pubsubevent); err != nil {
	// 	p.errors.Inc(channel)
	// 	p.logger.SpanError(span, err)
	// 	return err
	// } else {
	// 	p.logger.Debug("Unmarshalled for pubsub event is %s", pubsubevent)
	// 	defer p.HandleEvent(&pubsubevent)
	// }

	if len(body) == 0 {
		p.errors.Inc(channel)
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	p.logger.SpanDebug(span, "Body => %s", body)

	var WinEvents WinEventOriginalRequest
	if err := json.Unmarshal(body, &WinEvents); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}
	for _, event := range WinEvents.Events {
		t := time.UnixMilli(event.Timestamp)
		p.send(span, channel, event, &t)
	}

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

func (p *WinEventProcessor) send(span sreCommon.TracerSpan, channel string, event interface{}, t *time.Time) {
	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    event,
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

func NewWinEventProcessor(outputs *common.Outputs, observability *common.Observability) *WinEventProcessor {

	return &WinEventProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all WinEvent processor requests", []string{"channel"}, "WinEvent", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all WinEvent processor errors", []string{"channel"}, "WinEvent", "processor"),
	}
}
