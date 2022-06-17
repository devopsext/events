package output

import (
	"encoding/json"
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
	requests  sreCommon.Counter
	errors    sreCommon.Counter
}

func (g *GitlabOutput) Name() string {
	return "Gitlab"
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

// projects = TOKEN=PROJECT_ID@REF
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
			g.logger.SpanError(span, "Event data is empty")
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
			g.logger.SpanDebug(span, "Gitlab projects are not found")
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
			pair := strings.SplitN(project, "=", 2)
			token := g.options.Token

			if len(pair) == 2 && !utils.IsEmpty(pair[0]) {
				token = pair[0]
				project = pair[1]
			}

			pair = strings.SplitN(project, "@", 2)
			if len(pair) < 2 {
				continue
			}

			id := pair[0]
			ref := pair[1]
			if utils.IsEmpty(ref) {
				ref = "main"
			}

			g.requests.Inc(id, ref)

			opt := &gitlab.RunPipelineTriggerOptions{Ref: &ref, Token: &token, Variables: variables}
			pipeline, response, err := g.client.PipelineTriggers.RunPipelineTrigger(id, opt)
			if err != nil {
				g.errors.Inc(id, ref)
				g.logger.SpanError(span, err)
				continue
			}

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				g.errors.Inc(id, ref)
				g.logger.SpanError(span, "Gitlab reposne: %s", response.Status)
				continue
			}
			g.logger.SpanDebug(span, "Gitlab pipeline => %s", pipeline.WebURL)
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
		requests:  observability.Metrics().Counter("requests", "Count of all gitlab requests", []string{"project_id", "ref"}, "gitlab", "output"),
		errors:    observability.Metrics().Counter("errors", "Count of all gitlab errors", []string{"project_id", "ref"}, "gitlab", "output"),
	}
}
