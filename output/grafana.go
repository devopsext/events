package output

import (
	"context"
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
	Message            string
	AttributesSelector string
}

type GrafanaOutput struct {
	wg             *sync.WaitGroup
	message        *toolsRender.TextTemplate
	attributes     *toolsRender.TextTemplate
	options        GrafanaOutputOptions
	tracer         sreCommon.Tracer
	logger         sreCommon.Logger
	requests       sreCommon.Counter
	errors         sreCommon.Counter
	grafanaEventer *sreProvider.GrafanaEventer
	rateLimiter    *rate.Limiter
}

func (g *GrafanaOutput) Name() string {
	return "Grafana"
}

func (g *GrafanaOutput) getAttributes(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

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

	g.logger.SpanDebug(span, "Grafana raw attributes => %s", m)

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

		b, err := g.message.RenderObject(jsonObject)
		if err != nil {
			g.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			g.logger.SpanDebug(span, "Grafana message is empty")
			return
		}
		if err := g.rateLimiter.Wait(context.TODO()); err != nil {
			return
		}
		g.requests.Inc()
		g.logger.SpanDebug(span, "Grafana message => %s", message)

		attributes, err := g.getAttributes(jsonObject, span)
		if err != nil {
			g.logger.SpanError(span, err)
		}

		err = g.grafanaEventer.Interval(message, attributes, event.Time, event.Time)
		if err != nil {
			g.errors.Inc()
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
		tracer:         observability.Traces(),
		requests:       observability.Metrics().Counter("requests", "Count of all grafana requests", []string{}, "grafana", "output"),
		errors:         observability.Metrics().Counter("errors", "Count of all grafana errors", []string{}, "grafana", "output"),
		grafanaEventer: grafanaEventer,
	}
}
