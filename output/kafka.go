package output

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
)

var kafkaOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_kafka_output_count",
	Help: "Count of all kafka output count",
}, []string{"kafka_output_brokers", "kafka_output_topic"})

type KafkaOutputOptions struct {
	ClientID           string
	Template           string
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
	message  *render.TextTemplate
	options  KafkaOutputOptions
	tracer   common.Tracer
	logger   common.Logger
}

func (k *KafkaOutput) Send(event *common.Event) {

	k.wg.Add(1)
	go func() {
		defer k.wg.Done()

		if k.producer == nil || k.message == nil {
			k.logger.Error(errors.New("No producer or message"))
			return
		}

		if event == nil {
			k.logger.Error(errors.New("Event is empty"))
			return
		}

		span := k.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		b, err := k.message.Execute(event)
		if err != nil {
			k.logger.Error(err)
			span.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			k.logger.Debug("Message to Kafka is empty")
			return
		}

		k.logger.Debug("Message to Kafka => %s", message)

		(*k.producer).Input() <- &sarama.ProducerMessage{
			Topic: k.options.Topic,
			Value: sarama.ByteEncoder(b.Bytes()),
		}

		kafkaOutputCount.WithLabelValues(k.options.Brokers, k.options.Topic).Inc()
	}()
}

func makeKafkaProducer(wg *sync.WaitGroup, brokers string, topic string, config *sarama.Config, logger common.Logger) *sarama.AsyncProducer {

	brks := strings.Split(brokers, ",")
	if len(brks) == 0 || common.IsEmpty(brokers) {

		logger.Debug("Kafka brokers are not defined. Skipped.")
		return nil
	}

	if common.IsEmpty(topic) {

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

func NewKafkaOutput(wg *sync.WaitGroup, options KafkaOutputOptions, templateOptions render.TextTemplateOptions,
	logger common.Logger, tracer common.Tracer) *KafkaOutput {

	config := sarama.NewConfig()
	config.Version = sarama.V1_1_1_0

	if !common.IsEmpty(options.ClientID) {
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

	return &KafkaOutput{
		wg:       wg,
		producer: makeKafkaProducer(wg, options.Brokers, options.Topic, config, logger),
		message:  render.NewTextTemplate("kafka-message", options.Template, templateOptions, options, logger),
		options:  options,
		logger:   logger,
		tracer:   tracer,
	}
}

func init() {
	prometheus.Register(kafkaOutputCount)
}
