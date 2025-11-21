package processor

import (
	"bytes"
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

type GitlabProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
	hook    *gitlab.Webhook
}

type GitlabResponse struct {
	Message string
}

func GitlabProcessorType() string {
	return "Gitlab"
}

func (p *GitlabProcessor) EventType() string {
	return common.AsEventType(GitlabProcessorType())
}

func (p *GitlabProcessor) send(channel string, o interface{}, t *time.Time) {

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

func (p *GitlabProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("gitlab", "requests", "Count of all gitlab processor requests", labels, "processor")
	requests.Inc()

	p.outputs.Send(e)
	return nil
}

func (p *GitlabProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

	channel := strings.TrimLeft(r.URL.Path, "/")

	labels := make(map[string]string)
	labels["path"] = r.URL.Path
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("gitlab", "requests", "Count of all gitlab processor requests", labels, "processor")
	requests.Inc()

	errors := p.meter.Counter("gitlab", "errors", "Count of all gitlab processor errors", labels, "processor")

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

	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	events := []gitlab.Event{gitlab.PushEvents, gitlab.TagEvents, gitlab.IssuesEvents, gitlab.ConfidentialIssuesEvents, gitlab.CommentEvents,
		gitlab.MergeRequestEvents, gitlab.WikiPageEvents, gitlab.PipelineEvents, gitlab.BuildEvents, gitlab.JobEvents, gitlab.SystemHookEvents}
	payload, err := p.hook.Parse(r, events...)
	if err != nil {
		errors.Inc()
		p.logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	switch pl := payload.(type) {
	case gitlab.PushEventPayload:
		p.send(channel, payload.(gitlab.PushEventPayload), nil)
	case gitlab.TagEventPayload:
		p.send(channel, payload.(gitlab.TagEventPayload), nil)
	case gitlab.IssueEventPayload:
		event := payload.(gitlab.IssueEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.ConfidentialIssueEventPayload:
		event := payload.(gitlab.ConfidentialIssueEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.CommentEventPayload:
		event := payload.(gitlab.CommentEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.MergeRequestEventPayload:
		event := payload.(gitlab.MergeRequestEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.WikiPageEventPayload:
		event := payload.(gitlab.WikiPageEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.PipelineEventPayload:
		event := payload.(gitlab.PipelineEventPayload)
		p.send(channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.BuildEventPayload:
		event := payload.(gitlab.BuildEventPayload)
		p.send(channel, event, &event.BuildStartedAt.Time)
	case gitlab.JobEventPayload:
		event := payload.(gitlab.JobEventPayload)
		p.send(channel, event, &event.BuildStartedAt.Time)
	case gitlab.SystemHookPayload:
		p.send(channel, payload.(gitlab.SystemHookPayload), nil)
	default:
		p.logger.Debug("Not supported %s", pl)
	}

	response := &GitlabResponse{
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

func NewGitlabProcessor(outputs *common.Outputs, observability *common.Observability) *GitlabProcessor {

	logger := observability.Logs()
	hook, err := gitlab.New()
	if err != nil {
		logger.Debug("Gitlab processor is disabled.")
		return nil
	}

	return &GitlabProcessor{
		outputs: outputs,
		logger:  logger,
		hook:    hook,
		meter:   observability.Metrics(),
	}
}
