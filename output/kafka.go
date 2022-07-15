package output

import (
	"strings"
	"sync"
	"time"

	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"

	"github.com/devopsext/events/common"

	"github.com/Shopify/sarama"
)

type KafkaOutputOptions struct {
	ClientID           string
	Message            string
	Brokers            string
	Topic              string
	FlushFrequency     int
	FlushMaxMessages   int
	NetMaxOpenRequests int
	NetDialTimeout     int
	NetReadTimeout     int
	NetWriteTimeout    int
}

type KafkaOutput struct {
	wg       *sync.WaitGroup
	producer *sarama.AsyncProducer
	message  *toolsRender.TextTemplate
	options  KafkaOutputOptions
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

func (k *KafkaOutput) Name() string {
	return "Kafka"
}

func (k *KafkaOutput) Send(event *common.Event) {

	k.wg.Add(1)
	go func() {
		defer k.wg.Done()

		if event == nil {
			k.logger.Debug("Event is empty")
			return
		}

		span := k.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		b, err := k.message.RenderObject(event)
		if err != nil {
			k.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			k.logger.SpanDebug(span, "Kafka message is empty")
			return
		}

		k.requests.Inc(k.options.Topic)
		k.logger.SpanDebug(span, "Kafka  message => %s", message)

		(*k.producer).Input() <- &sarama.ProducerMessage{
			Topic: k.options.Topic,
			Value: sarama.ByteEncoder(b),
		}

		for err = range (*k.producer).Errors() {
			k.errors.Inc(k.options.Topic)
		}
	}()
}

func makeKafkaProducer(wg *sync.WaitGroup, brokers string, topic string, config *sarama.Config, logger sreCommon.Logger) *sarama.AsyncProducer {

	brks := strings.Split(brokers, ",")
	if len(brks) == 0 || utils.IsEmpty(brokers) {

		logger.Debug("Kafka brokers are not defined. Skipped.")
		return nil
	}

	if utils.IsEmpty(topic) {
		logger.Debug("Kafka topic is not defined. Skipped.")
		return nil
	}

	logger.Info("Start %s for %s...", config.ClientID, topic)

	producer, err := sarama.NewAsyncProducer(brks, config)
	if err != nil {
		logger.Error(err)
		return nil
	}
	return &producer
}

func NewKafkaOutput(wg *sync.WaitGroup, options KafkaOutputOptions, templateOptions toolsRender.TemplateOptions, observability *common.Observability) *KafkaOutput {

	config := sarama.NewConfig()
	config.Version = sarama.V1_1_1_0

	if !utils.IsEmpty(options.ClientID) {
		config.ClientID = options.ClientID
	}

	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Flush.Frequency = time.Second * time.Duration(options.FlushFrequency)
	config.Producer.Flush.MaxMessages = options.FlushMaxMessages

	config.Net.MaxOpenRequests = options.NetMaxOpenRequests
	config.Net.DialTimeout = time.Second * time.Duration(options.NetDialTimeout)
	config.Net.ReadTimeout = time.Second * time.Duration(options.NetReadTimeout)
	config.Net.WriteTimeout = time.Second * time.Duration(options.NetWriteTimeout)

	logger := observability.Logs()
	producer := makeKafkaProducer(wg, options.Brokers, options.Topic, config, logger)
	if producer == nil {
		logger.Error("no producer")
		return nil
	}

	messageOpts := toolsRender.TemplateOptions{
		Name:       "kafka-message",
		Content:    common.Content(options.Message),
		TimeFormat: templateOptions.TimeFormat,
	}
	message, err := toolsRender.NewTextTemplate(messageOpts)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return &KafkaOutput{
		wg:       wg,
		producer: producer,
		message:  message,
		options:  options,
		logger:   logger,
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all kafka requests", []string{"topic"}, "kafka", "output"),
		errors:   observability.Metrics().Counter("errors", "Count of all kafka errors", []string{"topic"}, "kafka", "output"),
	}
}
