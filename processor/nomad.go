package processor

import (
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	nomad "github.com/hashicorp/nomad/api"
)

const channel = "nomad"

type NomadProcessor struct {
	outputs *common.Outputs
	logger  sreCommon.Logger
	meter   sreCommon.Meter
}

func (p *NomadProcessor) HandleEvent(e *common.Event) error {
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	labels := make(map[string]string)
	labels["event_channel"] = e.Channel
	labels["processor"] = p.EventType()

	requests := p.meter.Counter("nomad", "requests", "Count of all nomad processor requests", labels, "processor")
	requests.Inc()

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
		meter:   observability.Metrics(),
	}
}
