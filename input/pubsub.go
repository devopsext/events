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
	tracer     sreCommon.Tracer
	logger     sreCommon.Logger
	requests   sreCommon.Counter
	errors     sreCommon.Counter
}

func (ps *PubSubInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		ps.logger.Info("Start pubsub input...")

		sub := ps.client.Subscription(ps.options.Subscription)
		ps.logger.Info("PubSub input is up. Listening...")

		err := sub.Receive(ps.ctx, func(ctx context.Context, m *pubsub.Message) {
			span := ps.tracer.StartSpan()
			defer m.Ack()
			defer span.Finish()

			ps.requests.Inc()
			ps.logger.SpanDebug(span, string(m.Data))

			var event common.Event
			if err := json.Unmarshal(m.Data, &event); err != nil {
				ps.errors.Inc()
				ps.logger.SpanError(span, err)
				return
			}

			p := ps.processors.Find(event.Type)
			if p == nil {
				ps.logger.SpanDebug(span, "PubSub processor is not found for %s", event.Type)
				return
			}

			event.SetLogger(ps.logger)
			event.SetSpanContext(span.GetContext())

			err := p.HandleEvent(&event)
			if err != nil {
				ps.errors.Inc()
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
	option.WithGRPCConnectionPool(2)

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, options.ProjectID, o)
	if err != nil {
		logger.Error(err)
		return nil
	}

	meter := observability.Metrics()

	return &PubSubInput{
		options:    options,
		client:     client,
		ctx:        ctx,
		processors: processors,
		eventer:    observability.Events(),
		tracer:     observability.Traces(),
		logger:     observability.Logs(),
		requests:   meter.Counter("pubsub", "requests", "Count of all pubsub input requests", map[string]string{}, "input", "pubsub"),
		errors:     meter.Counter("pubsub", "errors", "Count of all pubsub input errors", map[string]string{}, "input", "pubsub"),
	}
}
