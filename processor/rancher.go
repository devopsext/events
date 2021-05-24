package processor

import (
	"net/http"

	"github.com/devopsext/events/common"
)

type RancherProcessor struct {
	outputs *common.Outputs
	logger  common.Logger
	tracer  common.Tracer
}

func (p *RancherProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {
}

func NewRancherProcessor(outputs *common.Outputs, logger common.Logger, tracer common.Tracer) *RancherProcessor {
	return &RancherProcessor{
		outputs: outputs,
		logger:  logger,
		tracer:  tracer,
	}
}
