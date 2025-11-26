package processor

import (
	"encoding/json"
	errPkg "errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/go-playground/webhooks/v6/gitlab"
)

type TeamcityProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
	hook    *gitlab.Webhook
}

type TeamcityEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	BuildEvent     string    `json:"build_event"`
	BuildName      string    `json:"build_name"`
	BuildStatusUrl string    `json:"build_status_url,omitempty"`
	TriggeredBy    string    `json:"triggered_by,omitempty"`
	BuildResult    string    `json:"build_result,omitempty"`
	Target         string    `json:"target,omitempty"`
}

type TeamcityResponse struct {
	Message string
}

func TeamcityProcessorType() string {
	return "Teamcity"
}

func (p TeamcityProcessor) EventType() string {
	return common.AsEventType(TeamcityProcessorType())
}

func (p TeamcityProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("teamcity", "requests", "Count of all teamcity processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p TeamcityProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {
	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("teamcity", "requests", "Count of all teamcity processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("teamcity", "errors", "Count of all teamcity processor errors", labels, "processor")

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

	var tc TeamcityEvent
	if err := json.Unmarshal(body, &tc); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	p.send(channel, tc, &tc.Timestamp)

	response := &TeamcityResponse{
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

func (p TeamcityProcessor) send(channel string, tc TeamcityEvent, t *time.Time) {
	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    tc,
	}
	if t != nil && (*t).UnixNano() > 0 {
		e.SetTime((*t).UTC())
	} else {
		e.SetTime(time.Now().UTC())
	}
	e.SetLogger(p.logger)
	p.outputs.Send(e)
}

func NewTeamcityProcessor(outputs *common.Outputs, observability *common.Observability) *TeamcityProcessor {
	return &TeamcityProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
