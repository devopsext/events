package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	gitlab "github.com/xanzy/go-gitlab"
)

type GitlabOutputOptions struct {
	BaseURL   string
	Token     string
	Variables string
	Projects  string
}

type GitlabOutput struct {
	wg        *sync.WaitGroup
	client    *gitlab.Client
	projects  *render.TextTemplate
	variables *render.TextTemplate
	options   GitlabOutputOptions
	tracer    sreCommon.Tracer
	logger    sreCommon.Logger
	counter   sreCommon.Counter
}

func (g *GitlabOutput) getVariables(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

	attrs := make(map[string]string)
	if g.variables == nil {
		return attrs, nil
	}

	a, err := g.variables.Execute(o)
	if err != nil {
		return attrs, err
	}

	m := a.String()
	if utils.IsEmpty(m) {
		return attrs, nil
	}

	g.logger.SpanDebug(span, "Gitlab raw variables => %s", m)

	var object map[string]interface{}

	if err := json.Unmarshal([]byte(m), &object); err != nil {
		return attrs, err
	}

	for k, v := range object {
		vs, ok := v.(string)
		if ok {
			attrs[k] = vs
		}
	}
	return attrs, nil
}

// "https://some.host.domain/group/subgroup/project/-/pipelines/893667"
func (g *GitlabOutput) getProject(s string) string {

	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	arr := strings.Split(u.Path, "/-/")
	if len(arr) > 0 {
		return arr[0]
	}
	return ""
}

func (g *GitlabOutput) Send(event *common.Event) {

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		if g.client == nil || g.projects == nil {
			g.logger.Debug("No client or projects")
			return
		}

		if event == nil {
			g.logger.Debug("Event is empty")
			return
		}

		span := g.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			err := errors.New("event data is empty")
			g.logger.SpanError(span, err)
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			g.logger.SpanError(span, err)
			return
		}

		projects := ""
		if g.projects != nil {
			b, err := g.projects.Execute(jsonObject)
			if err != nil {
				g.logger.SpanDebug(span, err)
			} else {
				projects = b.String()
			}
		}

		if utils.IsEmpty(projects) {
			err := errors.New("gitlab projects are not found")
			g.logger.SpanError(span, err)
			return
		}

		variables, err := g.getVariables(jsonObject, span)
		if err != nil {
			g.logger.SpanError(span, err)
		}

		arr := strings.Split(projects, "\n")
		for _, project := range arr {

			project = strings.TrimSpace(project)
			if utils.IsEmpty(project) {
				continue
			}

			parr := strings.Split(project, "@")
			if len(parr) < 2 {
				continue
			}

			id := parr[0]
			ref := parr[1]
			if utils.IsEmpty(ref) {
				ref = "main"
			}

			opt := &gitlab.RunPipelineTriggerOptions{Ref: &ref, Token: &g.options.Token, Variables: variables}
			pipeline, response, err := g.client.PipelineTriggers.RunPipelineTrigger(id, opt)
			if err != nil {
				g.logger.SpanError(span, err)
				continue
			}

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				g.logger.SpanError(span, fmt.Errorf("gitlab reposne: %s", response.Status))
				continue
			}

			g.logger.SpanDebug(span, "Gitlab pipeline => %s", pipeline.WebURL)
			g.counter.Inc(g.getProject(pipeline.WebURL), pipeline.Ref)
		}
	}()
}

func NewGitlabOutput(wg *sync.WaitGroup,
	options GitlabOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability *common.Observability) *GitlabOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.BaseURL) {
		logger.Debug("Gitlab base URL is not defined. Skipped")
		return nil
	}

	client, err := gitlab.NewClient(options.Token, gitlab.WithBaseURL(options.BaseURL))
	if err != nil {
		logger.Error(err)
		return nil
	}

	return &GitlabOutput{
		wg:        wg,
		client:    client,
		projects:  render.NewTextTemplate("gitlab-projects", options.Projects, templateOptions, options, logger),
		variables: render.NewTextTemplate("gitlab-variables", options.Variables, templateOptions, options, logger),
		options:   options,
		logger:    logger,
		tracer:    observability.Traces(),
		counter:   observability.Metrics().Counter("requests", "Count of all gitlab request", []string{"project", "ref"}, "gitlab", "output"),
	}
}
