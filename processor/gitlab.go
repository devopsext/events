package processor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/go-playground/webhooks/v6/gitlab"
)

type GitlabProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
	hook     *gitlab.Webhook
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

func (p *GitlabProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

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

func (p *GitlabProcessor) HandleEvent(e *common.Event) error {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func (p *GitlabProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) error {

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

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		p.errors.Inc(channel)
		err := fmt.Errorf("Content-Type=%s, expect application/json", contentType)
		p.logger.SpanError(span, err)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return err
	}

	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	events := []gitlab.Event{gitlab.PushEvents, gitlab.TagEvents, gitlab.IssuesEvents, gitlab.ConfidentialIssuesEvents, gitlab.CommentEvents,
		gitlab.MergeRequestEvents, gitlab.WikiPageEvents, gitlab.PipelineEvents, gitlab.BuildEvents, gitlab.JobEvents, gitlab.SystemHookEvents}
	payload, err := p.hook.Parse(r, events...)
	if err != nil {
		p.errors.Inc(channel)
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	switch pl := payload.(type) {
	case gitlab.PushEventPayload:
		p.send(span, channel, payload.(gitlab.PushEventPayload), nil)
	case gitlab.TagEventPayload:
		p.send(span, channel, payload.(gitlab.TagEventPayload), nil)
	case gitlab.IssueEventPayload:
		event := payload.(gitlab.IssueEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.ConfidentialIssueEventPayload:
		event := payload.(gitlab.ConfidentialIssueEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.CommentEventPayload:
		event := payload.(gitlab.CommentEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.MergeRequestEventPayload:
		event := payload.(gitlab.MergeRequestEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.WikiPageEventPayload:
		event := payload.(gitlab.WikiPageEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.PipelineEventPayload:
		event := payload.(gitlab.PipelineEventPayload)
		p.send(span, channel, event, &event.ObjectAttributes.CreatedAt.Time)
	case gitlab.BuildEventPayload:
		event := payload.(gitlab.BuildEventPayload)
		p.send(span, channel, event, &event.BuildStartedAt.Time)
	case gitlab.JobEventPayload:
		event := payload.(gitlab.JobEventPayload)
		p.send(span, channel, event, &event.BuildStartedAt.Time)
	case gitlab.SystemHookPayload:
		p.send(span, channel, payload.(gitlab.SystemHookPayload), nil)
	default:
		p.logger.SpanDebug(span, "Not supported %s", pl)
	}

	response := &GitlabResponse{
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

func NewGitlabProcessor(outputs *common.Outputs, observability *common.Observability) *GitlabProcessor {

	logger := observability.Logs()
	hook, err := gitlab.New()
	if err != nil {
		logger.Debug("Gitlab processor is disabled.")
		return nil
	}

	return &GitlabProcessor{
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		hook:     hook,
		requests: observability.Metrics().Counter("requests", "Count of all gitlab processor requests", []string{"channel"}, "gitlab", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all gitlab processor errors", []string{"channel"}, "gitlab", "processor"),
	}
}
