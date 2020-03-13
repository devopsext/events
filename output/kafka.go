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

type KafkaOutput struct {
	wg       *sync.WaitGroup
	brokers  string
	topic    string
	producer *sarama.AsyncProducer
	template *render.TextTemplate
}

func (k *KafkaOutput) Send(o interface{}) {

	k.wg.Add(1)
	go func() {
		defer k.wg.Done()

		if k.producer == nil || k.template == nil {
			return
		}

		b, err := k.template.Execute(o)
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
			Topic: k.topic,
			Value: sarama.ByteEncoder(b.Bytes()),
		}

		kafkaOutputCount.WithLabelValues(k.brokers, k.topic).Inc()
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

func NewKafkaOutput(wg *sync.WaitGroup, clientID string, template string, brokers string, topic string,
	flushFrequency int, flushMaxMessages int, netMaxOpenRequests int, netDialTimeout int,
	netReadTimeout int, netWriteTimeout int) *KafkaOutput {

	config := sarama.NewConfig()
	config.Version = sarama.V1_1_1_0

	if !common.IsEmpty(clientID) {
		config.ClientID = clientID
	}

	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.Flush.Frequency = time.Second * time.Duration(flushFrequency)
	config.Producer.Flush.MaxMessages = flushMaxMessages

	config.Net.MaxOpenRequests = netMaxOpenRequests
	config.Net.DialTimeout = time.Second * time.Duration(netDialTimeout)
	config.Net.ReadTimeout = time.Second * time.Duration(netReadTimeout)
	config.Net.WriteTimeout = time.Second * time.Duration(netWriteTimeout)

	return &KafkaOutput{
		wg:       wg,
		brokers:  brokers,
		topic:    topic,
		producer: makeKafkaProducer(wg, brokers, topic, config),
		//template: render.NewTextTemplate("kafka", template),
	}
}

func init() {
	prometheus.Register(kafkaOutputCount)
}
