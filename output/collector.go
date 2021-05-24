package output

import (
	"errors"
	"net"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/client_golang/prometheus"
)

var collectorOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_collector_output_count",
	Help: "Count of all collector output count",
}, []string{"collector_output_address"})

type CollectorOutputOptions struct {
	Address  string
	Template string
}

type CollectorOutput struct {
	wg         *sync.WaitGroup
	options    CollectorOutputOptions
	connection *net.UDPConn
	message    *render.TextTemplate
	tracer     common.Tracer
	logger     common.Logger
}

func (c *CollectorOutput) Send(event *common.Event) {

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		if c.connection == nil || c.message == nil {
			c.logger.Error(errors.New("No connection or message"))
			return
		}

		if event == nil {
			c.logger.Error(errors.New("Event is empty"))
			return
		}

		span := c.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		b, err := c.message.Execute(event)
		if err != nil {
			c.logger.Error(err)
			span.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			c.logger.Debug("Message to Collector is empty")
			return
		}

		c.logger.Debug("Message to Collector => %s", message)

		_, err = c.connection.Write(b.Bytes())
		if err != nil {
			c.logger.Error(err)
			span.Error(err)
		}

		collectorOutputCount.WithLabelValues(c.options.Address).Inc()
	}()
}

func makeCollectorOutputConnection(address string, logger common.Logger) *net.UDPConn {

	if common.IsEmpty(address) {

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

func NewCollectorOutput(wg *sync.WaitGroup, options CollectorOutputOptions, templateOptions render.TextTemplateOptions,
	logger common.Logger, tracer common.Tracer) *CollectorOutput {

	return &CollectorOutput{
		wg:         wg,
		options:    options,
		message:    render.NewTextTemplate("collector-message", options.Template, templateOptions, options, logger),
		connection: makeCollectorOutputConnection(options.Address, logger),
		tracer:     tracer,
		logger:     logger,
	}
}

func init() {
	prometheus.Register(collectorOutputCount)
}
