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

type DataDogProcessor struct {
	outputs *common.Outputs
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	counter sreCommon.Counter
}

type DataDogEvent struct {
	Type  string `json:"type"`
	Msg   string `json:"msg"`
	Title string `json:"title"`
}

type DataDogAlert struct {
	ID         string `json:"id"`
	Metric     string `json:"metric"`
	Priority   string `json:"priority"`
	Query      string `json:"query"`
	Scope      string `json:"scope,omitempty"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	Transition string `json:"transition"`
	Type       string `json:"type"`
}

type DataDogIncident struct {
	Title string `json:"title"`
}

type DataDogMetric struct {
	Namespace string `json:"namespace"`
}

type DataDogSecurity struct {
	RuleName string `json:"rule_name"`
}

type DataDogOrg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DataDogRequest struct {
	ID          string           `json:"id"`
	Date        int              `json:"date"`
	LastUpdated int              `json:"last_updated"`
	Link        string           `json:"link"`
	Priority    string           `json:"priority"`
	Snapshot    string           `json:"snapshot"`
	Event       *DataDogEvent    `json:"event"`
	Alert       *DataDogAlert    `json:"alert,omitempty"`
	Incident    *DataDogIncident `json:"incident,omitempty"`
	Metric      *DataDogMetric   `json:"metric,omitempty"`
	Security    *DataDogSecurity `json:"security,omitempty"`
	Org         *DataDogOrg      `json:"org,omitempty"`
	Tags        string           `json:"tags,omitempty"`
	TextOnlyMsg string           `json:"text_only_msg,omitempty"`
	User        string           `json:"user,omitempty"`
	UserName    string           `json:"username,omitempty"`
}

type DataDogResponse struct {
	Message string
}

func DataDogProcessorType() string {
	return "DataDog"
}

func (p *DataDogProcessor) EventType() string {
	return common.AsEventType(DataDogProcessorType())
}

func (p *DataDogProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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
	p.counter.Inc(e.Channel)
}

func (p *DataDogProcessor) HandleEvent(e *common.Event) {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return
	}

	p.outputs.Send(e)
	p.counter.Inc(e.Channel)
}

func (p *DataDogProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		p.logger.SpanError(span, "Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return
	}

	var datadog DataDogRequest
	if err := json.Unmarshal(body, &datadog); err != nil {
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return
	}

	channel := strings.TrimLeft(r.URL.Path, "/")

	t := time.Unix(int64(datadog.LastUpdated), 0)
	p.send(span, channel, datadog, &t)

	response := &DataDogResponse{
		Message: "OK",
	}

	resp, err := json.Marshal(response)
	if err != nil {
		p.logger.SpanError(span, "Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func NewDataDogProcessor(outputs *common.Outputs, observability *common.Observability) *DataDogProcessor {

	return &DataDogProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		counter: observability.Metrics().Counter("requests", "Count of all datadog processor requests", []string{"channel"}, "datadog", "processor"),
	}
}
