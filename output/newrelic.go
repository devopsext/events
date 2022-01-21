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

type NewRelicOutputOptions struct {
	MessageTemplate string
}

type NewRelicOutput struct {
	wg              *sync.WaitGroup
	message         *render.TextTemplate
	options         NewRelicOutputOptions
	tracer          sreCommon.Tracer
	logger          sreCommon.Logger
	counter         sreCommon.Counter
	newrelicEventer *sreProvider.NewRelicEventer
}

func (r *NewRelicOutput) Send(event *common.Event) {

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		if r.message == nil {
			r.logger.Debug("No message")
			return
		}

		if event == nil {
			r.logger.Debug("Event is empty")
			return
		}

		span := r.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			err := errors.New("Event data is empty")
			r.logger.SpanError(span, err)
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			r.logger.SpanError(span, err)
			return
		}

		b, err := r.message.Execute(jsonObject)
		if err != nil {
			r.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			r.logger.SpanDebug(span, "NewRelic message is empty")
			return
		}

		r.logger.SpanDebug(span, "Message to NewRelic => %s", message)

		attributes := make(map[string]string)
		// how to determine attributes from jsonObject

		r.newrelicEventer.At(message, attributes, event.Time)
		r.counter.Inc()
	}()
}

func NewNewRelicOutput(wg *sync.WaitGroup,
	options NewRelicOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability common.Observability,
	newrelicEventer *sreProvider.NewRelicEventer) *NewRelicOutput {

	logger := observability.Logs()
	if newrelicEventer == nil {
		logger.Debug("NewRelic eventer is not defined. Skipped")
		return nil
	}

	return &NewRelicOutput{
		wg:              wg,
		message:         render.NewTextTemplate("newrelic-message", options.MessageTemplate, templateOptions, options, logger),
		options:         options,
		logger:          logger,
		tracer:          observability.Traces(),
		counter:         observability.Metrics().Counter("requests", "Count of all newrelic requests", []string{}, "newrelic", "output"),
		newrelicEventer: newrelicEventer,
	}
}
