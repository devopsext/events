package processor

import (
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	nomad "github.com/hashicorp/nomad/api"
	"time"
)

const channel = "nomad"

type NomadProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

func (p *NomadProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}
	p.requests.Inc(e.Channel)
	p.outputs.Send(e)
	return nil
}

func NomadProcessorType() string {
	return "Nomad"
}

func (p *NomadProcessor) EventType() string {
	return common.AsEventType(NomadProcessorType())
}

func (p *NomadProcessor) ProcessEvent(ne nomad.Event) error {
	ce := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    ne,
	}
	ce.SetTime(time.Now().UTC())
	err := p.HandleEvent(ce)
	if err != nil {
		return err
	}
	return nil
}

func NewNomadProcessor(outputs *common.Outputs, observability *common.Observability) *NomadProcessor {
	return &NomadProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		requests: observability.Metrics().Counter(
			"requests",
			"Count of all nomad processor requests",
			[]string{"channel"},
			channel,
			"processor",
		),
		errors: observability.Metrics().Counter(
			"errors",
			"Count of all nomad processor errors",
			[]string{"channel"},
			channel,
			"processor",
		),
	}
}
