package output

import (
	"errors"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
)

type NewRelicOutputOptions struct {
	MessageTemplate string
	AlertExpression string
}

type NewRelicOutput struct {
	wg      *sync.WaitGroup
	message *render.TextTemplate
	options NewRelicOutputOptions
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	counter sreCommon.Counter
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

		// URLs := t.options.URL
		// if t.selector != nil {

		// 	b, err := t.selector.Execute(jsonObject)
		// 	if err != nil {
		// 		t.logger.SpanDebug(span, err)
		// 	} else {
		// 		URLs = b.String()
		// 	}
		// }

		// if utils.IsEmpty(URLs) {
		// 	err := errors.New("Telegram URLs are not found")
		// 	t.logger.SpanError(span, err)
		// 	return
		// }

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

		// switch event.Type {
		// case "AlertmanagerEvent":
		// 	if err := r.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert)); err != nil {
		// 		t.sendErrorMessage(span.GetContext(), URL, message, err)
		// 	}
		// default:
		// 	t.sendMessage(span.GetContext(), URL, message)
		// }
	}()
}

func NewNewRelicOutput(wg *sync.WaitGroup,
	options NewRelicOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability common.Observability) *NewRelicOutput {

	logger := observability.Logs()
	// if utils.IsEmpty(options.URL) {
	// 	logger.Debug("Telegram URL is not defined. Skipped")
	// 	return nil
	// }

	return &NewRelicOutput{
		wg:      wg,
		message: render.NewTextTemplate("telegram-message", options.MessageTemplate, templateOptions, options, logger),
		options: options,
		logger:  logger,
		tracer:  observability.Traces(),
		counter: observability.Metrics().Counter("requests", "Count of all newrelic outputs", []string{}, "newrelic", "output"),
	}
}
