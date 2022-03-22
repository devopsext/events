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

type NewRelicOutputOptions struct {
	Message            string
	AttributesSelector string
}

type NewRelicOutput struct {
	wg              *sync.WaitGroup
	message         *render.TextTemplate
	attributes      *render.TextTemplate
	options         NewRelicOutputOptions
	tracer          sreCommon.Tracer
	logger          sreCommon.Logger
	counter         sreCommon.Counter
	newrelicEventer *sreProvider.NewRelicEventer
}

func (n *NewRelicOutput) getAttributes(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

	attrs := make(map[string]string)
	if n.attributes == nil {
		return attrs, nil
	}

	a, err := n.attributes.Execute(o)
	if err != nil {
		return attrs, err
	}

	m := a.String()
	if utils.IsEmpty(m) {
		return attrs, nil
	}

	n.logger.SpanDebug(span, "NewRelic raw attributes => %s", m)

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
			r.logger.SpanError(span, "Event data is empty")
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

		r.logger.SpanDebug(span, "NewRelic message => %s", message)

		attributes, err := r.getAttributes(jsonObject, span)
		if err != nil {
			r.logger.SpanError(span, err)
		}

		r.newrelicEventer.At(message, attributes, event.Time)
		r.counter.Inc()
	}()
}

func NewNewRelicOutput(wg *sync.WaitGroup,
	options NewRelicOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability *common.Observability,
	newrelicEventer *sreProvider.NewRelicEventer) *NewRelicOutput {

	logger := observability.Logs()
	if newrelicEventer == nil {
		logger.Debug("NewRelic eventer is not defined. Skipped")
		return nil
	}

	return &NewRelicOutput{
		wg:              wg,
		message:         render.NewTextTemplate("newrelic-message", options.Message, templateOptions, options, logger),
		attributes:      render.NewTextTemplate("newrelic-attributes", options.AttributesSelector, templateOptions, options, logger),
		options:         options,
		logger:          logger,
		tracer:          observability.Traces(),
		counter:         observability.Metrics().Counter("requests", "Count of all newrelic requests", []string{}, "newrelic", "output"),
		newrelicEventer: newrelicEventer,
	}
}
