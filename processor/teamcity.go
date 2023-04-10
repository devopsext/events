package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/go-playground/webhooks/v6/gitlab"
	"io"
	"net/http"
	"strings"
	"time"
)

type TeamcityProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
	hook     *gitlab.Webhook
}

type TeamcityRequest struct {
	Build struct {
		BuildStatus           string        `json:"buildStatus"`
		BuildResult           string        `json:"buildResult"`
		BuildResultPrevious   string        `json:"buildResultPrevious"`
		BuildResultDelta      string        `json:"buildResultDelta"`
		NotifyType            string        `json:"notifyType"`
		BuildFullName         string        `json:"buildFullName"`
		BuildName             string        `json:"buildName"`
		BuildId               string        `json:"buildId"`
		BuildTypeId           string        `json:"buildTypeId"`
		BuildInternalTypeId   string        `json:"buildInternalTypeId"`
		BuildExternalTypeId   string        `json:"buildExternalTypeId"`
		BuildStatusUrl        string        `json:"buildStatusUrl"`
		BuildStatusHtml       string        `json:"buildStatusHtml"`
		BuildStartTime        string        `json:"buildStartTime"`
		CurrentTime           string        `json:"currentTime"`
		RootUrl               string        `json:"rootUrl"`
		ProjectName           string        `json:"projectName"`
		ProjectId             string        `json:"projectId"`
		ProjectInternalId     string        `json:"projectInternalId"`
		ProjectExternalId     string        `json:"projectExternalId"`
		BuildNumber           string        `json:"buildNumber"`
		AgentName             string        `json:"agentName"`
		AgentOs               string        `json:"agentOs"`
		AgentHostname         string        `json:"agentHostname"`
		TriggeredBy           string        `json:"triggeredBy"`
		Message               string        `json:"message"`
		Text                  string        `json:"text"`
		BuildStateDescription string        `json:"buildStateDescription"`
		BuildIsPersonal       bool          `json:"buildIsPersonal"`
		DerivedBuildEventType string        `json:"derivedBuildEventType"`
		BuildRunners          []string      `json:"buildRunners"`
		BuildTags             []interface{} `json:"buildTags"`
		ExtraParameters       []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"extraParameters"`
		TeamcityProperties []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"teamcityProperties"`
		MaxChangeFileListSize          int           `json:"maxChangeFileListSize"`
		MaxChangeFileListCountExceeded bool          `json:"maxChangeFileListCountExceeded"`
		ChangeFileListCount            int           `json:"changeFileListCount"`
		Changes                        []interface{} `json:"changes"`
	} `json:"build"`
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
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p TeamcityProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {
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

	var tc TeamcityRequest
	if err := json.Unmarshal(body, &tc); err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return err
	}

	EventDateTime, err := time.Parse(time.RFC3339Nano, tc.Build.CurrentTime)
	if err != nil {
		p.send(span, channel, tc, nil)
		return nil
	}
	p.send(span, channel, tc, &EventDateTime)

	response := &TeamcityResponse{
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

func (p TeamcityProcessor) send(span sreCommon.TracerSpan, channel string, tc TeamcityRequest, t *time.Time) {
	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    tc.Build,
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

func NewTeamcityProcessor(outputs *common.Outputs, observability *common.Observability) *TeamcityProcessor {
	return &TeamcityProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all teamcity processor requests", []string{"channel"}, "zabbix", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all teamcity processor errors", []string{"channel"}, "zabbix", "processor"),
	}
}
