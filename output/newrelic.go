package output

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"
)

type NewRelicOutputOptions struct {
	Message            string
	AttributesSelector string
}

type NewRelicOutput struct {
	wg              *sync.WaitGroup
	message         *toolsRender.TextTemplate
	attributes      *toolsRender.TextTemplate
	options         NewRelicOutputOptions
	tracer          sreCommon.Tracer
	logger          sreCommon.Logger
	requests        sreCommon.Counter
	errors          sreCommon.Counter
	newrelicEventer *sreProvider.NewRelicEventer
}

func (n *NewRelicOutput) Name() string {
	return "NewRelic"
}

func (n *NewRelicOutput) getAttributes(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

	attrs := make(map[string]string)
	if n.attributes == nil {
		return attrs, nil
	}

	a, err := n.attributes.RenderObject(o)
	if err != nil {
		return attrs, err
	}

	m := string(a)
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

		b, err := r.message.RenderObject(jsonObject)
		if err != nil {
			r.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			r.logger.SpanDebug(span, "NewRelic message is empty")
			return
		}

		r.requests.Inc()
		r.logger.SpanDebug(span, "NewRelic message => %s", message)

		attributes, err := r.getAttributes(jsonObject, span)
		if err != nil {
			r.logger.SpanError(span, err)
		}

		err = r.newrelicEventer.At(message, attributes, event.Time)
		if err != nil {
			r.errors.Inc()
		}
	}()
}

func NewNewRelicOutput(wg *sync.WaitGroup,
	options NewRelicOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	observability *common.Observability,
	newrelicEventer *sreProvider.NewRelicEventer) *NewRelicOutput {

	logger := observability.Logs()
	if newrelicEventer == nil {
		logger.Debug("NewRelic eventer is not defined. Skipped")
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "newrelic-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts)
	if err != nil {
		logger.Error(err)
		return nil
	}

	attributesOpts := toolsRender.TemplateOptions{
		Name:       "newrelic-attributes",
		Content:    common.Content(options.AttributesSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	attributes, err := toolsRender.NewTextTemplate(attributesOpts)
	if err != nil {
		logger.Error(err)
	}

	return &NewRelicOutput{
		wg:              wg,
		message:         message,
		attributes:      attributes,
		options:         options,
		logger:          logger,
		tracer:          observability.Traces(),
		requests:        observability.Metrics().Counter("requests", "Count of all newrelic requests", []string{}, "newrelic", "output"),
		errors:          observability.Metrics().Counter("errors", "Count of all newrelic errors", []string{}, "newrelic", "output"),
		newrelicEventer: newrelicEventer,
	}
}
