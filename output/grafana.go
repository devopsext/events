package output

import (
	"context"
	"encoding/json"
	"golang.org/x/time/rate"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	"github.com/devopsext/utils"
)

type GrafanaOutputOptions struct {
	Message            string
	AttributesSelector string
}

type GrafanaOutput struct {
	wg             *sync.WaitGroup
	message        *render.TextTemplate
	attributes     *render.TextTemplate
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

	a, err := g.attributes.Execute(o)
	if err != nil {
		return attrs, err
	}

	m := a.String()
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

		if g.message == nil {
			g.logger.Debug("No message")
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

		b, err := g.message.Execute(jsonObject)
		if err != nil {
			g.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(b.String())
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
	templateOptions render.TextTemplateOptions,
	observability *common.Observability,
	grafanaEventer *sreProvider.GrafanaEventer) *GrafanaOutput {

	logger := observability.Logs()
	if grafanaEventer == nil {
		logger.Debug("Grafana eventer is not defined. Skipped")
		return nil
	}

	return &GrafanaOutput{
		rateLimiter:    rate.NewLimiter(rate.Every(time.Minute/time.Duration(30)), 1),
		wg:             wg,
		message:        render.NewTextTemplate("grafana-message", options.Message, templateOptions, options, logger),
		attributes:     render.NewTextTemplate("grafana-attributes", options.AttributesSelector, templateOptions, options, logger),
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		requests:       observability.Metrics().Counter("requests", "Count of all grafana requests", []string{}, "grafana", "output"),
		errors:         observability.Metrics().Counter("errors", "Count of all grafana errors", []string{}, "grafana", "output"),
		grafanaEventer: grafanaEventer,
	}
}
