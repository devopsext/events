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
	meter   sreCommon.Meter
}

func RancherProcessorType() string {
	return "Rancher"
}

func (p *RancherProcessor) EventType() string {
	return common.AsEventType(RancherProcessorType())
}

func (p *RancherProcessor) HandleEvent(e *common.Event) {
}

func (p *RancherProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {
}

func NewRancherProcessor(outputs *common.Outputs, observability *common.Observability) *RancherProcessor {
	return &RancherProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		meter:   observability.Metrics(),
	}
}
