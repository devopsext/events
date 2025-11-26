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
	EventID      string `json:"EventRecordID"`
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
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

func WinEventProcessorType() string {
	return "Win"
}

func (p *WinEventProcessor) EventType() string {
	return common.AsEventType(WinEventProcessorType())
}

func (p *WinEventProcessor) HandleEvent(e *common.Event) error {

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("winevent", "requests", "Count of all winevent processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("winevent", "errors", "Count of all winevent processor errors", labels, "processor")

	if e == nil {
		errors.Inc()
		p.logger.Debug("Event is not defined")
		return nil
	}
	if js, err := e.JsonBytes(); err == nil {
		var PubSubEvents WinEventPubsubRequest
		if err := json.Unmarshal(js, &PubSubEvents); err != nil {
			errors.Inc()
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
			t := time.UnixMilli(event.Timestamp * 1000)
			newEvent.SetTime(t.UTC())
			requests.Inc()
			p.outputs.Send(newEvent)
		}
	}
	return nil
}

func (p *WinEventProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("winevent", "requests", "Count of all winevent processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("winevent", "errors", "Count of all winevent processor errors", labels, "processor")

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

	var WinEvents WinEventOriginalRequest
	if err := json.Unmarshal(body, &WinEvents); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}
	for _, event := range WinEvents.Events {
		t := time.UnixMilli(event.Timestamp)
		p.send(channel, event, &t)
	}

	response := &WinEventResponse{
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

func (p *WinEventProcessor) send(channel string, event interface{}, t *time.Time) {
	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    event,
	}
	if t != nil && t.UnixNano() > 0 {
		e.SetTime(t.UTC())
	} else {
		e.SetTime(time.Now().UTC())
	}
	e.SetLogger(p.logger)
	p.outputs.Send(e)
}

func NewWinEventProcessor(outputs *common.Outputs, observability *common.Observability) *WinEventProcessor {

	return &WinEventProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
