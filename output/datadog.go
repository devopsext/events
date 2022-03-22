package output

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	"github.com/devopsext/utils"
)

type DataDogOutputOptions struct {
	Message            string
	AttributesSelector string
}

type DataDogOutput struct {
	wg             *sync.WaitGroup
	message        *render.TextTemplate
	attributes     *render.TextTemplate
	options        DataDogOutputOptions
	tracer         sreCommon.Tracer
	logger         sreCommon.Logger
	counter        sreCommon.Counter
	datadogEventer *sreProvider.DataDogEventer
}

func (d *DataDogOutput) getAttributes(o interface{}, span sreCommon.TracerSpan) (map[string]string, error) {

	attrs := make(map[string]string)
	if d.attributes == nil {
		d.logger.Debug("output attributes not set")
		return attrs, nil
	}

	a, err := d.attributes.Execute(o)
	if err != nil {
		d.logger.Debug("error execute attributes template: %s", err)
		return attrs, err
	}

	m := a.String()
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

		b, err := d.message.Execute(jsonObject)
		if err != nil {
			d.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			d.logger.SpanDebug(span, "DataDog message is empty")
			return
		}

		d.logger.SpanDebug(span, "DataDog message => %s", message)

		attributes, err := d.getAttributes(jsonObject, span)
		if err != nil {
			d.logger.SpanError(span, err)
		}
		d.logger.Debug("Message: %s, Attributes: %s, Time: %s", message, strings.Join(sreCommon.MapToArray(attributes), ","), event.Time.Format(time.RFC822))

		d.datadogEventer.At(message, attributes, event.Time)
		d.counter.Inc()
	}()
}

func NewDataDogOutput(wg *sync.WaitGroup,
	options DataDogOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability *common.Observability,
	datadogEventer *sreProvider.DataDogEventer) *DataDogOutput {

	logger := observability.Logs()
	if datadogEventer == nil {
		logger.Debug("DataDog eventer is not defined. Skipped")
		return nil
	}

	return &DataDogOutput{
		wg:             wg,
		message:        render.NewTextTemplate("datadog-message", options.Message, templateOptions, options, logger),
		attributes:     render.NewTextTemplate("datadog-attributes", options.AttributesSelector, templateOptions, options, logger),
		options:        options,
		logger:         logger,
		tracer:         observability.Traces(),
		counter:        observability.Metrics().Counter("requests", "Count of all datadog requests", []string{}, "datadog", "output"),
		datadogEventer: datadogEventer,
	}
}
