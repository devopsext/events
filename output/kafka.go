package output

import (
	"strings"
	"sync"
	"time"

	sreCommon "github.com/devopsext/sre/common"
	toolsRender "github.com/devopsext/tools/render"
	"github.com/devopsext/utils"

	"github.com/devopsext/events/common"

	"github.com/IBM/sarama"
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
	logger   sreCommon.Logger
	meter    sreCommon.Meter
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

		b, err := k.message.RenderObject(event)
		if err != nil {
			k.logger.Error(err)
			return
		}

		message := strings.TrimSpace(string(b))
		if utils.IsEmpty(message) {
			k.logger.Debug("Kafka message is empty")
			return
		}

		labels := make(map[string]string)
		labels["event_channel"] = event.Channel
		labels["event_type"] = event.Type
		labels["kafka_client_id"] = k.options.ClientID
		labels["kafka_brokers"] = k.options.Brokers
		labels["kafka_topic"] = k.options.Topic
		labels["output"] = k.Name()

		requests := k.meter.Counter("kafka", "requests", "Count of all kafka requests", labels, "output")
		requests.Inc()
		k.logger.Debug("Kafka  message => %s", message)

		(*k.producer).Input() <- &sarama.ProducerMessage{
			Topic: k.options.Topic,
			Value: sarama.ByteEncoder(b),
		}

		for err = range (*k.producer).Errors() {
			errors := k.meter.Counter("kafka", "errors", "Count of all kafka errors", labels, "output")
			errors.Inc()
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
	message, err := toolsRender.NewTextTemplate(messageOpts, observability)
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
		meter:    observability.Metrics(),
	}
}
