package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

// TODO: move to config
const (
	prometheusAPIURL = "http://prometheus.example.com/api/v1/query"
	queryTemplate    = "avg(sre_availability{product='%s'}) by (cluster, service) < 99" // Example query: change this to your own query
)

type DataDogProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type DataDogEvent struct {
	Type  string `json:"type"`
	Msg   string `json:"msg"`
	Title string `json:"title"`
}

type DataDogAlert struct {
	ID         string   `json:"id"`
	Metric     string   `json:"metric"`
	Priority   string   `json:"priority"`
	Query      string   `json:"query"`
	Scope      string   `json:"scope,omitempty"`
	Status     string   `json:"status"`
	Title      string   `json:"title"`
	Transition string   `json:"transition"`
	Type       string   `json:"type"`
	Services   []string `json:"services,omitempty"`
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
	Date        int64            `json:"date"`
	LastUpdated int64            `json:"last_updated"`
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

type PromQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
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
}

func (p *DataDogProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *DataDogProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	channel := strings.TrimLeft(r.URL.Path, "/")
	p.requests.Inc(channel)

	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
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

	var datadog DataDogRequest
	if err := json.Unmarshal(body, &datadog); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	t := time.UnixMilli(datadog.LastUpdated)

	scopes := strings.Split(datadog.Alert.Scope, ",")
	for _, scope := range scopes {
		tag := strings.Split(scope, ":")
		if len(tag) == 2 {
			if tag[0] == "product" {
				datadog.Alert.Services = p.getAffectedServicesByProduct(tag[1])
				break
			}
		}
	}

	p.send(span, channel, datadog, &t)

	response := &DataDogResponse{
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

func NewDataDogProcessor(outputs *common.Outputs, observability *common.Observability) *DataDogProcessor {

	return &DataDogProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all datadog processor requests", []string{"channel"}, "datadog", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all datadog processor errors", []string{"channel"}, "datadog", "processor"),
	}
}

func (p *DataDogProcessor) getAffectedServicesByProduct(product string) []string {
	res := make([]string, 0)
	product = strings.ToUpper(product)

	u, err := url.Parse(prometheusAPIURL)
	if err != nil {
		p.logger.Error("Failed to parse Prometheus API URL: %s", err)
		return res
	}

	q := u.Query()
	q.Set("query", fmt.Sprintf(queryTemplate, product))
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		p.logger.Error("Failed to fetch from Prometheus API: %s", err)
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		p.logger.Error("Received non-200 response: %d %s", resp.StatusCode, resp.Status)
		return res
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Error("Failed to read response body: %s", err)
		return res
	}

	var response PromQueryResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		p.logger.Error("Failed to unmarshal response: %s", err)
		return res
	}

	if response.Status != "success" {
		p.logger.Error("Unsuccessful query: %s", response.Status)
		return res
	}

	for _, result := range response.Data.Result {
		res = append(res, fmt.Sprintf("%s:%s", result.Metric["cluster"], result.Metric["service"]))
		p.logger.Debug("Metric: %v\n", result.Metric)
	}

	return res
}
