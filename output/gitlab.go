package output

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"
	"github.com/xanzy/go-gitlab"
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
	projects  *toolsRender.TextTemplate
	variables *toolsRender.TextTemplate
	options   GitlabOutputOptions
	logger    sreCommon.Logger
	meter     sreCommon.Meter
}

func (g *GitlabOutput) Name() string {
	return "Gitlab"
}

func (g *GitlabOutput) getVariables(o interface{}) (map[string]string, error) {

	attrs := make(map[string]string)
	if g.variables == nil {
		return attrs, nil
	}

	a, err := g.variables.RenderObject(o)
	if err != nil {
		return attrs, err
	}

	m := string(a)
	if utils.IsEmpty(m) {
		return attrs, nil
	}

	g.logger.Debug("Gitlab raw variables => %s", m)

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
/*func (g *GitlabOutput) getProject(s string) string {

	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	arr := strings.Split(u.Path, "/-/")
	if len(arr) > 0 {
		return arr[0]
	}
	return ""
}*/

// projects = TOKEN=PROJECT_ID@REF
func (g *GitlabOutput) Send(event *common.Event) {

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		if event == nil {
			g.logger.Debug("Event is empty")
			return
		}

		if event.Data == nil {
			g.logger.Error("Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			g.logger.Error(err)
			return
		}

		projects := ""
		if g.projects != nil {
			b, err := g.projects.RenderObject(jsonObject)
			if err != nil {
				g.logger.Debug(err)
			} else {
				projects = string(b)
			}
		}

		if utils.IsEmpty(projects) {
			g.logger.Debug("Gitlab projects are not found")
			return
		}

		variables, err := g.getVariables(jsonObject)
		if err != nil {
			g.logger.Error(err)
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

			labels := make(map[string]string)
			labels["event_channel"] = event.Channel
			labels["event_type"] = event.Type
			labels["gitlab_project_id"] = project
			labels["gitlab_ref"] = ref
			labels["output"] = g.Name()

			requests := g.meter.Counter("gitlab", "requests", "Count of all gitlab requests", labels, "output")
			requests.Inc()

			opt := &gitlab.RunPipelineTriggerOptions{Ref: &ref, Token: &token, Variables: variables}
			pipeline, response, err := g.client.PipelineTriggers.RunPipelineTrigger(id, opt)

			errors := g.meter.Counter("gitlab", "errors", "Count of all gitlab errors", labels, "output")
			if err != nil {
				errors.Inc()
				g.logger.Error(err)
				continue
			}

			if response.StatusCode < 200 || response.StatusCode >= 300 {
				errors.Inc()
				g.logger.Error("Gitlab response: %s", response.Status)
				continue
			}
			g.logger.Debug("Gitlab pipeline => %s", pipeline.WebURL)
		}
	}()
}

func NewGitlabOutput(wg *sync.WaitGroup,
	options GitlabOutputOptions,
	templateOptions toolsRender.TemplateOptions,
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

	projectsOpts := toolsRender.TemplateOptions{
		Name:       "gitlab-projects",
		Content:    common.Content(options.Projects),
		TimeFormat: templateOptions.TimeFormat,
	}
	projects, err := toolsRender.NewTextTemplate(projectsOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	variablesOpts := toolsRender.TemplateOptions{
		Name:       "gitlab-variables",
		Content:    common.Content(options.Variables),
		TimeFormat: templateOptions.TimeFormat,
	}
	variables, err := toolsRender.NewTextTemplate(variablesOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &GitlabOutput{
		wg:        wg,
		client:    client,
		projects:  projects,
		variables: variables,
		options:   options,
		logger:    logger,
		meter:     observability.Metrics(),
	}
}
