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

type CloudflareProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

type CloudflareRequest struct {
	Text string `json:"text"`
}

type CloudflareResponse struct {
	Message string
}

func CloudflareProcessorType() string {
	return "Cloudflare"
}

func (p *CloudflareProcessor) EventType() string {
	return common.AsEventType(CloudflareProcessorType())
}

func (p *CloudflareProcessor) send(channel string, o interface{}, t *time.Time) {

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

func (p *CloudflareProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("cloudflare", "requests", "Count of all cloudflare processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *CloudflareProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("cloudflare", "requests", "Count of all cloudflare processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("cloudflare", "errors", "Count of all cloudflare processor errors", labels, "processor")

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

	var Cloudflare CloudflareRequest
	if err := json.Unmarshal(body, &Cloudflare); err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	p.send(channel, Cloudflare, nil)

	response := &CloudflareResponse{
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

func NewCloudflareProcessor(outputs *common.Outputs, observability *common.Observability) *CloudflareProcessor {

	return &CloudflareProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		meter:   observability.Metrics(),
	}
}
