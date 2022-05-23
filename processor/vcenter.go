package processor

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type VCenterProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type VCenterResponse struct {
	Message string
}

func VCenterProcessorType() string {
	return "vcenter"
}

func (p *VCenterProcessor) EventType() string {
	return common.AsEventType(VCenterProcessorType())
}

func (p *VCenterProcessor) prepareStatus(status string) string {
	return strings.Title(strings.ToLower(status))
}

func (p *VCenterProcessor) send(span sreCommon.TracerSpan, channel string, data string) {

	for _, alert := range data {

		e := &common.Event{
			Channel: channel,
			Type:    p.EventType(),
			Data:    alert,
		}
		//e.SetTime(alert..UTC())
		if span != nil {
			e.SetSpanContext(span.GetContext())
			e.SetLogger(p.logger)
		}
		p.outputs.Send(e)
	}
}

func (p *VCenterProcessor) HandleEvent(e *common.Event) error {

	//	p.requests.Inc(e.Channel)
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	jsonString := e.Data.(string)
	var result map[string]interface{}
	json.Unmarshal([]byte(jsonString), &result)

	//data := result["data"].(map[string]interface{})

	eventTime, _ := time.Parse(time.RFC3339Nano, result["time"].(string))

	curevent := &common.Event{
		Data:    result,
		Channel: e.Channel,
		Type:    "vcenterEvent",
	}
	curevent.SetLogger(p.logger)
	curevent.SetTime(eventTime)

	p.outputs.Send(curevent)

	return nil
}
func NewVCenterProcessor(outputs *common.Outputs, observability *common.Observability) *VCenterProcessor {

	return &VCenterProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all google processor requests", []string{"channel"}, "aws", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all google processor errors", []string{"channel"}, "aws", "processor"),
	}
}
