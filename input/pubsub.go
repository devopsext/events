package input

import (
	"sync"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type PubSubInputOptions struct {
	Credentials  string
	ProjectID    string
	Subscription string
}

type PubSubInput struct {
	options PubSubInputOptions
	eventer sreCommon.Eventer
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	meter   sreCommon.Meter
	counter sreCommon.Counter
}

func (ps *PubSubInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {

	wg.Add(1)
	go func(wg *sync.WaitGroup) {

		defer wg.Done()
		ps.logger.Info("Start pubsub input...")

	}(wg)
}

func NewPubSubInput(options PubSubInputOptions, observability common.Observability) *PubSubInput {

	meter := observability.Metrics()
	return &PubSubInput{
		options: options,
		eventer: observability.Events(),
		tracer:  observability.Traces(),
		logger:  observability.Logs(),
		meter:   meter,
		counter: meter.Counter("requests", "Count of all pubsub input requests", []string{}, "pubsub", "input"),
	}
}
