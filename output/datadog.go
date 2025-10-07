package output

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"
)

type DataDogOutputOptions struct {
	Name               string
	Message            string
	AttributesSelector string
}

type DataDogOutput struct {
	wg             *sync.WaitGroup
	name           *toolsRender.TextTemplate
	message        *toolsRender.TextTemplate
	attributes     *toolsRender.TextTemplate
	options        DataDogOutputOptions
	tracer         sreCommon.Tracer
	logger         sreCommon.Logger
	requests       sreCommon.Counter
	errors         sreCommon.Counter
	datadogEventer *sreProvider.DataDogEventer
}

func (d *DataDogOutput) Name() string {
	return "DataDog"
}

func (d *DataDogOutput) getAttributes(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

	attrs := make(map[string]string)
	if d.attributes == nil {
		d.logger.Debug("output attributes not set")
		return attrs, nil
	}

	a, err := d.attributes.RenderObject(o)
	if err != nil {
		d.logger.Debug("error execute attributes template: %s", err)
		return attrs, err
	}

	m := string(a)
	if utils.IsEmpty(m) {
		d.logger.Debug("attributes are empty")
		return attrs, nil
	}

	d.logger.SpanDebug(span, "DataDog raw attributes => %s", m)

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

func (d *DataDogOutput) Send(event *common.Event) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		if event == nil {
			d.logger.Debug("Event is empty")
			return
		}

		span := d.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			d.logger.SpanError(span, "Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			d.logger.SpanError(span, err)
			return
		}

		a, err := d.name.RenderObject(jsonObject)
		if err != nil {
			d.logger.SpanError(span, err)
			return
		}

		name := strings.TrimSpace(string(a))
		if utils.IsEmpty(a) {
			d.logger.SpanDebug(span, "DataDog name is empty")
			return
		}

		b, err := d.message.RenderObject(jsonObject)
		if err != nil {
			d.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			d.logger.SpanDebug(span, "DataDog message is empty")
			return
		}

		d.requests.Inc()
		d.logger.SpanDebug(span, "DataDog message => %s%s", name, message)

		attributes, err := d.getAttributes(jsonObject, span)
		if err != nil {
			d.logger.SpanError(span, err)
		}
		d.logger.Debug("Name: %s, Message: %s, Attributes: %s, Time: %s", name, message, strings.Join(utils.MapToArray(attributes), ","), event.Time.Format(time.RFC822))

		err = d.datadogEventer.At(name, message, attributes, event.Time)
		if err != nil {
			d.errors.Inc()
		}
	}()
}

func NewDataDogOutput(wg *sync.WaitGroup,
	options DataDogOutputOptions,
	templateOptions toolsRender.TemplateOptions,
	observability *common.Observability,
	datadogEventer *sreProvider.DataDogEventer) *DataDogOutput {

	logger := observability.Logs()
	if datadogEventer == nil {
		logger.Debug("DataDog eventer is not defined. Skipped")
		return nil
	}

	nameOpts := toolsRender.TemplateOptions{
		Name:       "datadog-name",
		Content:    common.Content(options.Name),
		TimeFormat: templateOptions.TimeFormat,
	}
	name, err := toolsRender.NewTextTemplate(nameOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "datadog-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	attributesOpts := toolsRender.TemplateOptions{
		Name:       "datadog-attributes",
		Content:    common.Content(options.AttributesSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	attributes, err := toolsRender.NewTextTemplate(attributesOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &DataDogOutput{
		wg:             wg,
		name:           name,
		message:        message,
		attributes:     attributes,
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		requests:       observability.Metrics().Counter("datadog", "requests", "Count of all datadog requests", map[string]string{}, "output", "datadog"),
		errors:         observability.Metrics().Counter("datadog", "errors", "Count of all datadog errors", map[string]string{}, "output", "datadog"),
		datadogEventer: datadogEventer,
	}
}
