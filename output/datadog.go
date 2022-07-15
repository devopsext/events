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
	Message            string
	AttributesSelector string
}

type DataDogOutput struct {
	wg             *sync.WaitGroup
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

		if d.message == nil {
			d.logger.Debug("No message")
			return
		}

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
		d.logger.SpanDebug(span, "DataDog message => %s", message)

		attributes, err := d.getAttributes(jsonObject, span)
		if err != nil {
			d.logger.SpanError(span, err)
		}
		d.logger.Debug("Message: %s, Attributes: %s, Time: %s", message, strings.Join(utils.MapToArray(attributes), ","), event.Time.Format(time.RFC822))

		err = d.datadogEventer.At(message, attributes, event.Time)
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

	messageOpts := toolsRender.TemplateOptions{
		Name:       "datadog-message",
		Content:    options.Message,
		TimeFormat: templateOptions.TimeFormat,
	}

	attributesOpts := toolsRender.TemplateOptions{
		Name:       "datadog-attributes",
		Content:    options.AttributesSelector,
		TimeFormat: templateOptions.TimeFormat,
	}

	return &DataDogOutput{
		wg:             wg,
		message:        toolsRender.NewTextTemplate(messageOpts),
		attributes:     toolsRender.NewTextTemplate(attributesOpts),
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		requests:       observability.Metrics().Counter("requests", "Count of all datadog requests", []string{}, "datadog", "output"),
		errors:         observability.Metrics().Counter("errors", "Count of all datadog errors", []string{}, "datadog", "output"),
		datadogEventer: datadogEventer,
	}
}
