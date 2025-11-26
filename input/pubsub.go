package input

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"cloud.google.com/go/pubsub"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	"google.golang.org/api/option"
)

type PubSubInputOptions struct {
	Credentials  string
	ProjectID    string
	Subscription string
}

type PubSubInput struct {
	options    PubSubInputOptions
	client     *pubsub.Client
	ctx        context.Context
	processors *common.Processors
	eventer    sreCommon.Eventer
	logger     sreCommon.Logger
	meter      sreCommon.Meter
}

func (ps *PubSubInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		ps.logger.Info("Start pubsub input...")

		sub := ps.client.Subscription(ps.options.Subscription)
		ps.logger.Info("PubSub input is up. Listening...")

		err := sub.Receive(ps.ctx, func(ctx context.Context, m *pubsub.Message) {

			labels := make(map[string]string)
			labels["subscription"] = sub.String()
			labels["input"] = "pubsub"

			requests := ps.meter.Counter("pubsub", "requests", "Count of all pubsub input requests", labels, "input")
			errors := ps.meter.Counter("pubsub", "errors", "Count of all pubsub input errors", labels, "input")

			requests.Inc()

			var event common.Event
			if err := json.Unmarshal(m.Data, &event); err != nil {
				errors.Inc()
				return
			}

			p := ps.processors.Find(event.Type)
			if p == nil {
				ps.logger.Debug("PubSub processor is not found for %s", event.Type)
				return
			}

			event.SetLogger(ps.logger)

			err := p.HandleEvent(&event)
			if err != nil {
				errors.Inc()
			}
		})

		if err != nil {
			ps.logger.Error(err)
		}
	}(wg)
}

func NewPubSubInput(options PubSubInputOptions, processors *common.Processors, observability *common.Observability) *PubSubInput {
	logger := observability.Logs()
	if utils.IsEmpty(options.Credentials) || utils.IsEmpty(options.ProjectID) || utils.IsEmpty(options.Subscription) {
		logger.Debug("PubSub input credentials, project ID or subscription is not defined. Skipped")
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

	return &PubSubInput{
		options:    options,
		client:     client,
		ctx:        ctx,
		processors: processors,
		eventer:    observability.Events(),
		logger:     observability.Logs(),
		meter:      observability.Metrics(),
	}
}
