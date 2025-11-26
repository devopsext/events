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
	Name               string
	Message            string
	AttributesSelector string
}

type NewRelicOutput struct {
	wg              *sync.WaitGroup
	name            *toolsRender.TextTemplate
	message         *toolsRender.TextTemplate
	attributes      *toolsRender.TextTemplate
	options         NewRelicOutputOptions
	logger          sreCommon.Logger
	meter           sreCommon.Meter
	newrelicEventer *sreProvider.NewRelicEventer
	newrelicOptions *sreProvider.NewRelicOptions
}

func (n *NewRelicOutput) Name() string {
	return "NewRelic"
}

func (n *NewRelicOutput) getAttributes(o interface{}) (map[string]string, error) {

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

	n.logger.Debug("NewRelic raw attributes => %s", m)

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

		if event.Data == nil {
			r.logger.Error("Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			r.logger.Error(err)
			return
		}

		b, err := r.message.RenderObject(jsonObject)
		if err != nil {
			r.logger.Error(err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			r.logger.Debug("NewRelic message is empty")
			return
		}

		labels := make(map[string]string)
		labels["event_channel"] = event.Channel
		labels["event_type"] = event.Type
		labels["newrelic_environment"] = r.newrelicOptions.Environment
		labels["newrelic_service_name"] = r.newrelicOptions.ServiceName
		labels["output"] = r.Name()

		requests := r.meter.Counter("newrelic", "requests", "Count of all newrelic requests", labels, "output")
		requests.Inc()
		r.logger.Debug("NewRelic message => %s", message)

		attributes, err := r.getAttributes(jsonObject)
		if err != nil {
			r.logger.Error(err)
		}

		err = r.newrelicEventer.At(message, "", attributes, event.Time)
		if err != nil {
			errors := r.meter.Counter("newrelic", "errors", "Count of all newrelic errors", labels, "output")
			errors.Inc()
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
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	attributesOpts := toolsRender.TemplateOptions{
		Name:       "newrelic-attributes",
		Content:    common.Content(options.AttributesSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	attributes, err := toolsRender.NewTextTemplate(attributesOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &NewRelicOutput{
		wg:              wg,
		message:         message,
		attributes:      attributes,
		options:         options,
		logger:          logger,
		meter:           observability.Metrics(),
		newrelicEventer: newrelicEventer,
	}
}
