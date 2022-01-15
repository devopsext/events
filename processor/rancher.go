package processor

import (
	"net/http"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type RancherProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	tracer  sreCommon.Tracer
}

func (p *RancherProcessor) Type() string {
	return "RancherEvent"
}

func (p *RancherProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {
}

func NewRancherProcessor(outputs *common.Outputs, logger sreCommon.Logger, tracer sreCommon.Tracer) *RancherProcessor {
	return &RancherProcessor{
		outputs: outputs,
		logger:  logger,
		tracer:  tracer,
	}
}
