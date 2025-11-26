package output

import (
	"net"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"
)

type CollectorOutputOptions struct {
	Address string
	Message string
}

type CollectorOutput struct {
	wg         *sync.WaitGroup
	options    CollectorOutputOptions
	connection *net.UDPConn
	message    *toolsRender.TextTemplate
	logger     sreCommon.Logger
	meter      sreCommon.Meter
}

func (c *CollectorOutput) Name() string {
	return "Collector"
}

func (c *CollectorOutput) Send(event *common.Event) {

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		if event == nil {
			c.logger.Debug("Event is empty")
			return
		}

		b, err := c.message.RenderObject(event)
		if err != nil {
			c.logger.Error(err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			c.logger.Debug("Collector message is empty")
			return
		}

		labels := make(map[string]string)
		labels["event_channel"] = event.Channel
		labels["event_type"] = event.Type
		labels["collector_address"] = c.options.Address
		labels["output"] = c.Name()

		requests := c.meter.Counter("collector", "requests", "Count of all collector requests", labels, "output")
		requests.Inc()

		c.logger.Debug("Collector message => %s", message)

		_, err = c.connection.Write(b)
		if err != nil {
			errors := c.meter.Counter("collector", "errors", "Count of all collector errors", labels, "output")
			errors.Inc()
			c.logger.Error(err)
		}
	}()
}

func makeCollectorOutputConnection(address string, logger sreCommon.Logger) *net.UDPConn {

	if utils.IsEmpty(address) {
		logger.Debug("Collector address is not defined. Skipped.")
		return nil
	}

	localAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		logger.Error(err)
		return nil
	}

	serverAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		logger.Error(err)
		return nil
	}

	connection, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return connection
}

func NewCollectorOutput(wg *sync.WaitGroup, options CollectorOutputOptions,
	templateOptions toolsRender.TemplateOptions, observability *common.Observability) *CollectorOutput {

	logger := observability.Logs()
	connection := makeCollectorOutputConnection(options.Address, logger)
	if connection == nil {
		logger.Error("no connection")
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "collector-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return &CollectorOutput{
		wg:         wg,
		options:    options,
		message:    message,
		connection: connection,
		logger:     logger,
		meter:      observability.Metrics(),
	}
}
