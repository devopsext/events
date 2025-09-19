package input

import (
	"context"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/processor"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
	nomad "github.com/hashicorp/nomad/api"
)

type NomadInputOptions struct {
	Address string
	Token   string
	Topics  []string
}

type NomadInput struct {
	options    NomadInputOptions
	client     *nomad.Client
	ctx        context.Context
	processors *common.Processors
	eventer    sreCommon.Eventer
	tracer     sreCommon.Tracer
	logger     sreCommon.Logger
	requests   sreCommon.Counter
	errors     sreCommon.Counter
}

func (n *NomadInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		p := n.processors.Find("NomadEvent").(*processor.NomadProcessor)
		if p == nil {
			n.logger.Debug("Nomad processor is not found for NomadEvent")
			return
		}

		n.logger.Info("Start nomad input...")

		topics := make(map[nomad.Topic][]string)
		for _, topic := range n.options.Topics {
			topics[nomad.Topic(topic)] = []string{"*"}
		}

		q := &nomad.QueryOptions{}

		stream := n.client.EventStream()

		for {
			eventCh, err := stream.Stream(n.ctx, topics, 0, q)
			if err != nil {
				n.logger.Error(err)
				time.Sleep(5 * time.Second)
				continue
			}

			chanOk := true
			for {
				select {
				case es, ok := <-eventCh:
					if !ok {
						n.logger.Error("Stream channel closed, restarting")
						chanOk = false
						time.Sleep(2 * time.Second)
						break
					}
					if es.Err != nil {
						n.logger.Error("Stream channel return error '%v', restarting", es.Err)
						chanOk = false
						time.Sleep(2 * time.Second)
						break
					}
					for _, ne := range es.Events {
						err := p.ProcessEvent(ne)
						if err != nil {
							n.logger.Error(err)
						}
					}
				}
				if !chanOk {
					break
				}
			}
			n.logger.Debug("Restart nomad input...")
		}
	}(wg)
}

func NewNomadInput(options NomadInputOptions, processors *common.Processors, observability *common.Observability) *NomadInput {
	logger := observability.Logs()
	if utils.IsEmpty(options.Address) || utils.IsEmpty(options.Token) {
		logger.Debug("Nomad input address or token is not defined. Skipped")
		return nil
	}

	config := nomad.DefaultConfig()
	config.Address = options.Address
	config.SecretID = options.Token
	client, err := nomad.NewClient(config)
	if err != nil {
		logger.Error(err)
		return nil
	}

	meter := observability.Metrics()

	return &NomadInput{
		options:    options,
		client:     client,
		processors: processors,
		ctx:        context.Background(),
		eventer:    observability.Events(),
		tracer:     observability.Traces(),
		logger:     observability.Logs(),
		requests:   meter.Counter("nomad", "requests", "Count of all nomad input requests", map[string]string{}, "input"),
		errors:     meter.Counter("nomad", "errors", "Count of all nomad input errors", map[string]string{}, "input"),
	}
}
