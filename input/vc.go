package input

import (
	"context"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/vcenter"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
)

type VCInputOptions struct {
	URL           string
	InsecureSSL   bool
	Checkpoint    bool
	AuthType      string
	AuthBasicName string
	AuthBasicPass string
	RootCA        string
	CheckpointDir string
}

type VCInput struct {
	options    VCInputOptions
	provider   *vcenter.EventStream
	processors *common.Processors
	ctx        context.Context
	tracer     sreCommon.Tracer
	logger     sreCommon.Logger
	meter      sreCommon.Meter
	requests   sreCommon.Counter
	errors     sreCommon.Counter
}

func (vcenter *VCInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		vcenter.logger.Info("Start vcenter input...")
		vcenter.provider.Stream(vcenter.ctx, outputs)
	}(wg)
}

func NewVCInput(options VCInputOptions, processors *common.Processors, observability *common.Observability) *VCInput {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) || utils.IsEmpty(options.AuthBasicName) || utils.IsEmpty(options.AuthBasicPass) {
		logger.Debug("VC input credentials or URL is not defined. Skipped")
		return nil
	}

	ctx := context.Background()

	var prov *vcenter.EventStream
	var err error
	prov, err = vcenter.NewEventStream(ctx, logger, vcenter.VCInputOptions(options))
	if err != nil {
		logger.Panic("could not connect to vCenter: %v", err)
	}
	logger.Info("connected vcenter")

	//log.Infow("connecting to vCenter", "address", cfg.EventProvider.VCenter.Address)
	//client, err := pubsub.NewClient(ctx, options.ProjectID, o)
	//if err != nil {
	//	logger.Error(err)
	//	return nil
	//}

	meter := observability.Metrics()
	return &VCInput{
		options:    options,
		processors: processors,
		tracer:     observability.Traces(),
		logger:     observability.Logs(),
		provider:   prov,
		ctx:        ctx,
		meter:      meter,
		requests:   meter.Counter("requests", "Count of all vc input requests", []string{"url"}, "http", "input"),
		errors:     meter.Counter("errors", "Count of all vc input errors", []string{"url"}, "http", "input"),
	}
}
