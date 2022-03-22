package output

import (
	"context"
	"os"
	"strings"
	"sync"

	pubsub "cloud.google.com/go/pubsub"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"google.golang.org/api/option"
)

type PubSubOutputOptions struct {
	Credentials   string
	ProjectID     string
	Message       string
	TopicSelector string
}

type PubSubOutput struct {
	wg       *sync.WaitGroup
	client   *pubsub.Client
	ctx      context.Context
	message  *render.TextTemplate
	selector *render.TextTemplate
	options  PubSubOutputOptions
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	counter  sreCommon.Counter
}

func (ps *PubSubOutput) Send(event *common.Event) {

	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()

		if ps.client == nil || ps.message == nil {
			ps.logger.Debug("No client or message")
			return
		}

		if event == nil {
			ps.logger.Debug("Event is empty")
			return
		}

		span := ps.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			ps.logger.SpanError(span, "Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			ps.logger.SpanError(span, err)
			return
		}

		topics := ""
		if ps.selector != nil {
			b, err := ps.selector.Execute(jsonObject)
			if err != nil {
				ps.logger.SpanDebug(span, err)
			} else {
				topics = b.String()
			}
		}

		if utils.IsEmpty(topics) {
			ps.logger.SpanError(span, "PubSub topics are not found")
			return
		}

		b, err := ps.message.Execute(jsonObject)
		if err != nil {
			ps.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			ps.logger.SpanDebug(span, "PubSub message is empty")
			return
		}

		ps.logger.SpanDebug(span, "PubSub message => %s", message)

		arr := strings.Split(topics, "\n")
		for _, topic := range arr {

			topic = strings.TrimSpace(topic)
			if utils.IsEmpty(topic) {
				continue
			}

			t := ps.client.Topic(topic)
			serverID, err := t.Publish(ps.ctx, &pubsub.Message{Data: []byte(message)}).Get(ps.ctx)
			if err != nil {
				ps.logger.SpanError(span, err)
				continue
			}
			ps.logger.SpanDebug(span, "PubSub server ID => %s", serverID)
			ps.counter.Inc(topic)
		}

	}()
}

func NewPubSubOutput(wg *sync.WaitGroup,
	options PubSubOutputOptions,
	templateOptions render.TextTemplateOptions,
	observability *common.Observability) *PubSubOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.Credentials) || utils.IsEmpty(options.ProjectID) {
		logger.Debug("PubSub output credentials or project ID is not defined. Skipped")
		return nil
	}

	var o option.ClientOption
	if _, err := os.Stat(options.Credentials); err == nil {
		o = option.WithCredentialsFile(options.Credentials)
	} else {
		o = option.WithCredentialsJSON([]byte(options.Credentials))
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, options.ProjectID, o)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return &PubSubOutput{
		wg:       wg,
		client:   client,
		ctx:      ctx,
		message:  render.NewTextTemplate("pubsub-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("pubsub-selector", options.TopicSelector, templateOptions, options, logger),
		options:  options,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all pubsub request", []string{"topic"}, "pubsub", "output"),
	}
}
