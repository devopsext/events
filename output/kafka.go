package output

import (
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
	template *render.TextTemplate
	options  KafkaOutputOptions
}

func (k *KafkaOutput) Send(event *common.Event) {

	k.wg.Add(1)
	go func() {
		defer k.wg.Done()

		if k.producer == nil || k.template == nil {
			return
		}

		b, err := k.template.Execute(event)
		if err != nil {

			log.Error(err)
			return
		}

		message := b.String()

		if common.IsEmpty(message) {

			log.Debug("Message to Kafka is empty")
			return
		}

		log.Debug("Message to Kafka => %s", message)

		(*k.producer).Input() <- &sarama.ProducerMessage{
			Topic: k.options.Topic,
			Value: sarama.ByteEncoder(b.Bytes()),
		}

		kafkaOutputCount.WithLabelValues(k.options.Brokers, k.options.Topic).Inc()
	}()
}

func makeKafkaProducer(wg *sync.WaitGroup, brokers string, topic string, config *sarama.Config) *sarama.AsyncProducer {

	brks := strings.Split(brokers, ",")
	if len(brks) == 0 || common.IsEmpty(brokers) {

		log.Debug("Kafka brokers are not defined. Skipped.")
		return nil
	}

	if common.IsEmpty(topic) {

		log.Debug("Kafka topic is not defined. Skipped.")
		return nil
	}

	log.Info("Start %s for %s...", config.ClientID, topic)

	producer, err := sarama.NewAsyncProducer(brks, config)
	if err != nil {
		log.Error(err)
		return nil
	}

	return &producer
}

func NewKafkaOutput(wg *sync.WaitGroup, options KafkaOutputOptions, templateOptions render.TextTemplateOptions) *KafkaOutput {

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
		producer: makeKafkaProducer(wg, options.Brokers, options.Topic, config),
		template: render.NewTextTemplate("kafka-template", options.Template, templateOptions, options),
		options:  options,
	}
}

func init() {
	prometheus.Register(kafkaOutputCount)
}
