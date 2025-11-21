package output

import (
	"context"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/pubsub"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
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
	message  *toolsRender.TextTemplate
	selector *toolsRender.TextTemplate
	options  PubSubOutputOptions
	meter    sreCommon.Meter
	logger   sreCommon.Logger
}

func (ps *PubSubOutput) Name() string {
	return "PubSub"
}

func (ps *PubSubOutput) Send(event *common.Event) {

	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()

		if event == nil {
			ps.logger.Debug("Event is empty")
			return
		}

		if event.Data == nil {
			ps.logger.Error("Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			ps.logger.Error(err)
			return
		}

		topics := ""
		if ps.selector != nil {
			b, err := ps.selector.RenderObject(jsonObject)
			if err != nil {
				ps.logger.Debug(err)
			} else {
				topics = string(b)
			}
		}

		if utils.IsEmpty(topics) {
			ps.logger.Error("PubSub topics are not found")
			return
		}

		b, err := ps.message.RenderObject(jsonObject)
		if err != nil {
			ps.logger.Error(err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			ps.logger.Debug("PubSub message is empty")
			return
		}

		ps.logger.Debug("PubSub message => %s", message)

		arr := strings.Split(topics, "\n")
		for _, topic := range arr {
			topic = strings.TrimSpace(topic)
			if utils.IsEmpty(topic) {
				continue
			}

			labels := make(map[string]string)
			labels["event_channel"] = event.Channel
			labels["event_type"] = event.Type
			labels["pubsub_project_id"] = ps.options.ProjectID
			labels["pubsub_topic"] = ps.options.TopicSelector
			labels["output"] = ps.Name()

			requests := ps.meter.Counter("pubsub", "requests", "Count of all pubsub requests", labels, "output")
			requests.Inc()

			t := ps.client.Topic(topic)
			serverID, err := t.Publish(ps.ctx, &pubsub.Message{Data: []byte(message)}).Get(ps.ctx)
			if err != nil {
				errors := ps.meter.Counter("pubsub", "errors", "Count of all pubsub errors", labels, "output")
				errors.Inc()
				ps.logger.Error(err)
				continue
			}
			ps.logger.Debug("PubSub server ID => %s", serverID)
		}
	}()
}

func NewPubSubOutput(wg *sync.WaitGroup,
	options PubSubOutputOptions,
	templateOptions toolsRender.TemplateOptions,
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

	messageOpts := toolsRender.TemplateOptions{
		Name:       "pubsub-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	selectorOpts := toolsRender.TemplateOptions{
		Name:       "pubsub-selector",
		Content:    common.Content(options.TopicSelector),
		TimeFormat: templateOptions.TimeFormat,
	}
	selector, err := toolsRender.NewTextTemplate(selectorOpts, observability)
	if err != nil {
		logger.Error(err)
	}

	return &PubSubOutput{
		wg:       wg,
		client:   client,
		ctx:      ctx,
		message:  message,
		selector: selector,
		options:  options,
		logger:   logger,
		meter:    observability.Metrics(),
	}
}
