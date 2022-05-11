package output

import (
	"encoding/json"
	"sync"

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
	counter        sreCommon.Counter
	grafanaEventer *sreProvider.GrafanaEventer
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

		message := b.String()
		if utils.IsEmpty(message) {
			g.logger.SpanDebug(span, "Grafana message is empty")
			return
		}

		g.logger.SpanDebug(span, "Grafana message => %s", message)

		attributes, err := g.getAttributes(jsonObject, span)
		if err != nil {
			g.logger.SpanError(span, err)
		}

		g.grafanaEventer.At(message, attributes, event.Time)
		g.counter.Inc()
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
		wg:             wg,
		message:        render.NewTextTemplate("grafana-message", options.Message, templateOptions, options, logger),
		attributes:     render.NewTextTemplate("grafana-attributes", options.AttributesSelector, templateOptions, options, logger),
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		counter:        observability.Metrics().Counter("requests", "Count of all grafana request", []string{}, "grafana", "output"),
		grafanaEventer: grafanaEventer,
	}
}
