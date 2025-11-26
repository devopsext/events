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

type Site24x7Processor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type Site24x7Request struct {
	MonitorID            int64  `json:"MONITOR_ID"`
	MonitorDashboardLink string `json:"MONITOR_DASHBOARD_LINK"`
	MonitorType          string `json:"MONITORTYPE"`
	MonitorName          string `json:"MONITORNAME"`
	MonitorURL           string `json:"MONITORURL"`
	MonitorGroupName     string `json:"MONITOR_GROUPNAME"`

	IncidentReason  string `json:"INCIDENT_REASON"`
	IncidentTime    string `json:"INCIDENT_TIME"`
	IncidentTimeISO string `json:"INCIDENT_TIME_ISO"`

	PollFrequency   int      `json:"POLLFREQUENCY"`
	Status          string   `json:"STATUS"`
	FailedLocations string   `json:"FAILED_LOCATIONS"`
	GroupTags       []string `json:"GROUP_TAGS,omitempty"`
	Tags            []string `json:"TAGS,omitempty"`
}

type Site24x7Response struct {
	Message string
}

func Site24x7ProcessorType() string {
	return "Site24x7"
}

func (p *Site24x7Processor) EventType() string {
	return common.AsEventType(Site24x7ProcessorType())
}

func (p *Site24x7Processor) send(channel string, o interface{}, t *time.Time) {

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

func (p *Site24x7Processor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("site24x7", "requests", "Count of all site24x7 processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *Site24x7Processor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("site24x7", "requests", "Count of all site24x7 processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("site24x7", "errors", "Count of all site24x7 processor errors", labels, "processor")

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

	var site24x7 Site24x7Request
	if err := json.Unmarshal(body, &site24x7); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	// 2022-03-18T01:51:19-0700"
	t, err := time.Parse("2006-01-02T15:04:05-0700", site24x7.IncidentTimeISO)
	if err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error incident time ISO format", http.StatusInternalServerError)
		return err
	}
	p.send(channel, site24x7, &t)

	response := &Site24x7Response{
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

func NewSite24x7Processor(outputs *common.Outputs, observability *common.Observability) *Site24x7Processor {

	return &Site24x7Processor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
