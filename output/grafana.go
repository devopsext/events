package output

import (
	"errors"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	"github.com/devopsext/utils"
)

type GrafanaOutputOptions struct {
	MessageTemplate string
}

type GrafanaOutput struct {
	wg             *sync.WaitGroup
	message        *render.TextTemplate
	options        GrafanaOutputOptions
	tracer         sreCommon.Tracer
	logger         sreCommon.Logger
	counter        sreCommon.Counter
	grafanaEventer *sreProvider.GrafanaEventer
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
			err := errors.New("Event data is empty")
			g.logger.SpanError(span, err)
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

		g.logger.SpanDebug(span, "Message to Grafana => %s", message)

		attributes := make(map[string]string)
		// how to determine attributes from jsonObject

		g.grafanaEventer.At(message, attributes, event.Time)
		g.counter.Inc()
	}()
}

func NewGrafanaOutput(wg *sync.WaitGroup,
	options GrafanaOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability common.Observability,
	grafanaEventer *sreProvider.GrafanaEventer) *GrafanaOutput {

	logger := observability.Logs()
	if grafanaEventer == nil {
		logger.Debug("Grafana eventer is not defined. Skipped")
		return nil
	}

	return &GrafanaOutput{
		wg:             wg,
		message:        render.NewTextTemplate("grafana-message", options.MessageTemplate, templateOptions, options, logger),
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		counter:        observability.Metrics().Counter("requests", "Count of all grafana request", []string{}, "grafana", "output"),
		grafanaEventer: grafanaEventer,
	}
}
