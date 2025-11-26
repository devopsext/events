package output

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"
)

type GrafanaOutputOptions struct {
	Name               string
	Message            string
	AttributesSelector string
}

type GrafanaOutput struct {
	wg             *sync.WaitGroup
	message        *toolsRender.TextTemplate
	attributes     *toolsRender.TextTemplate
	options        GrafanaOutputOptions
	logger         sreCommon.Logger
	meter          sreCommon.Meter
	grafanaEventer *sreProvider.GrafanaEventer
	rateLimiter    *rate.Limiter
}

func (g *GrafanaOutput) Name() string {
	return "Grafana"
}

func (g *GrafanaOutput) getAttributes(o interface{}) (map[string]string, error) {

	attrs := make(map[string]string)
	if g.attributes == nil {
		return attrs, nil
	}

	a, err := g.attributes.RenderObject(o)
	if err != nil {
		return attrs, err
	}

	m := string(a)
	if utils.IsEmpty(m) {
		return attrs, nil
	}

	g.logger.Debug("Grafana raw attributes => %s", m)

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

func (g *GrafanaOutput) Send(event *common.Event) {

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

		b, err := g.message.RenderObject(jsonObject)
		if err != nil {
			g.logger.Error(err)
			return
		}
		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			g.logger.Debug("Grafana message is empty")
			return
		}
		g.logger.Debug("Grafana message => %s", message)

		attributes, err := g.getAttributes(jsonObject)
		if err != nil {
			g.logger.Error(err)
		}
		g.logger.Debug("Grafana attributes => %s", attributes)

		labels := make(map[string]string)
		labels["event_channel"] = event.Channel
		labels["event_type"] = event.Type
		labels["output"] = g.Name()

		rateLimiterIn := g.meter.Counter("grafana", "rl_in", "Count of all grafana requests before waiting", labels, "output", "rate_limiter")
		rateLimiterIn.Inc()
		r := g.rateLimiter.Reserve()
		// TODO increment another counter events_grafana_output_ratelimiter_wait_time_total by r.Delay*time.Millisecond
		time.Sleep(r.Delay())

		rateLimiterOut := g.meter.Counter("grafana", "rl_out", "Count of all grafana requests after waiting", labels, "output", "rate_limiter")
		rateLimiterOut.Inc()

		requests := g.meter.Counter("grafana", "requests", "Count of all grafana requests", labels, "output")
		requests.Inc()

		err = g.grafanaEventer.Interval(message, message, attributes, event.Time, event.Time)
		if err != nil {
			errors := g.meter.Counter("grafana", "errors", "Count of all grafana errors", labels, "output")
			errors.Inc()
		}
	}()
}

func NewGrafanaOutput(wg *sync.WaitGroup,
	options GrafanaOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	observability *common.Observability,
	grafanaEventer *sreProvider.GrafanaEventer) *GrafanaOutput {

	logger := observability.Logs()
	if grafanaEventer == nil {
		logger.Debug("Grafana eventer is not defined. Skipped")
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "grafana-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	selectorOpts := toolsRender.TemplateOptions{
		Name:       "grafana-attributes",
		Content:    common.Content(options.AttributesSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	attributes, err := toolsRender.NewTextTemplate(selectorOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &GrafanaOutput{
		rateLimiter:    rate.NewLimiter(rate.Every(time.Minute/time.Duration(30)), 1),
		wg:             wg,
		message:        message,
		attributes:     attributes,
		options:        options,
		logger:         logger,
		meter:          observability.Metrics(),
		grafanaEventer: grafanaEventer,
	}
}
