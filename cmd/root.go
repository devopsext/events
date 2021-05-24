package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"

	"sync"
	"syscall"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/input"
	"github.com/devopsext/events/output"
	"github.com/devopsext/events/provider"
	"github.com/devopsext/events/render"
	utils "github.com/devopsext/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var VERSION = "unknown"

//var log = utils.GetLog()

var env = utils.GetEnvironment()
var logs = common.NewLogs()
var traces = common.NewTraces()
var stdout *provider.Stdout

type RootOptions struct {
	Logs    []string
	Metrics string
	Traces  []string

	PrometheusURL    string
	PrometheusListen string
}

var rootOptions = RootOptions{

	Logs:    strings.Split(env.Get("EVENTS_LOGS", "stdout").(string), ","),
	Metrics: env.Get("EVENTS_METRICS", "prometheus").(string),
	Traces:  strings.Split(env.Get("EVENTS_TRACES", "").(string), ","),

	PrometheusURL:    env.Get("EVENTS_PROMETHEUS_URL", "/metrics").(string),
	PrometheusListen: env.Get("EVENTS_PROMETHEUS_LISTEN", "127.0.0.1:8080").(string),
}

var textTemplateOptions = render.TextTemplateOptions{

	TimeFormat: env.Get("EVENTS_TEMPLATE_TIME_FORMAT", "2006-01-02T15:04:05.999Z").(string),
}

var stdoutOptions = provider.StdoutOptions{

	Format:   env.Get("EVENTS_STDOUT_FORMAT", "text").(string),
	Level:    env.Get("EVENTS_STDOUT_LEVEL", "info").(string),
	Template: env.Get("EVENTS_STDOUT_TEMPLATE", "{{.file}} {{.msg}}").(string),
}

var httpInputOptions = input.HttpInputOptions{

	K8sURL:          env.Get("EVENTS_HTTP_K8S_URL", "").(string),
	RancherURL:      env.Get("EVENTS_HTTP_RANCHER_URL", "").(string),
	AlertmanagerURL: env.Get("EVENTS_HTTP_ALERTMANAGER_URL", "").(string),
	Listen:          env.Get("EVENTS_HTTP_LISTEN", ":80").(string),
	Tls:             env.Get("EVENTS_HTTP_TLS", false).(bool),
	Cert:            env.Get("EVENTS_HTTP_CERT", "").(string),
	Key:             env.Get("EVENTS_HTTP_KEY", "").(string),
	Chain:           env.Get("EVENTS_HTTP_CHAIN", "").(string),
}

var collectorOutputOptions = output.CollectorOutputOptions{

	Address:  env.Get("EVENTS_COLLECTOR_ADDRESS", "").(string),
	Template: env.Get("EVENTS_COLLECTOR_MESSAGE_TEMPLATE", "").(string),
}

var kafkaOutputOptions = output.KafkaOutputOptions{

	ClientID:           env.Get("EVENTS_KAFKA_CLIEND_ID", "events_kafka").(string),
	Template:           env.Get("EVENTS_KAFKA_MESSAGE_TEMPLATE", "").(string),
	Brokers:            env.Get("EVENTS_KAFKA_BROKERS", "").(string),
	Topic:              env.Get("EVENTS_KAFKA_TOPIC", "events").(string),
	FlushFrequency:     env.Get("EVENTS_KAFKA_FLUSH_FREQUENCY", 1).(int),
	FlushMaxMessages:   env.Get("EVENTS_KAFKA_FLUSH_MAX_MESSAGES", 100).(int),
	NetMaxOpenRequests: env.Get("EVENTS_KAFKA_NET_MAX_OPEN_REQUESTS", 5).(int),
	NetDialTimeout:     env.Get("EVENTS_KAFKA_NET_DIAL_TIMEOUT", 30).(int),
	NetReadTimeout:     env.Get("EVENTS_KAFKA_NET_READ_TIMEOUT", 30).(int),
	NetWriteTimeout:    env.Get("EVENTS_KAFKA_NET_WRITE_TIMEOUT", 30).(int),
}

var telegramOutputOptions = output.TelegramOutputOptions{

	MessageTemplate:     env.Get("EVENTS_TELEGRAM_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate:    env.Get("EVENTS_TELEGRAM_SELECTOR_TEMPLATE", "").(string),
	URL:                 env.Get("EVENTS_TELEGRAM_URL", "").(string),
	Timeout:             env.Get("EVENTS_TELEGRAM_TIMEOUT", 30).(int),
	AlertExpression:     env.Get("EVENTS_TELEGRAM_ALERT_EXPRESSION", "g0.expr").(string),
	DisableNotification: env.Get("EVENTS_TELEGRAM_DISABLE_NOTIFICATION", "false").(string),
}

var slackOutputOptions = output.SlackOutputOptions{

	MessageTemplate:  env.Get("EVENTS_SLACK_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate: env.Get("EVENTS_SLACK_SELECTOR_TEMPLATE", "").(string),
	URL:              env.Get("EVENTS_SLACK_URL", "").(string),
	Timeout:          env.Get("EVENTS_SLACK_TIMEOUT", 30).(int),
	AlertExpression:  env.Get("EVENTS_SLACK_ALERT_EXPRESSION", "g0.expr").(string),
}

var workchatOutputOptions = output.WorkchatOutputOptions{

	MessageTemplate:  env.Get("EVENTS_WORKCHAT_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate: env.Get("EVENTS_WORKCHAT_SELECTOR_TEMPLATE", "").(string),
	URL:              env.Get("EVENTS_WORKCHAT_URL", "").(string),
	Timeout:          env.Get("EVENTS_WORKCHAT_TIMEOUT", 30).(int),
	AlertExpression:  env.Get("EVENTS_WORKCHAT_ALERT_EXPRESSION", "g0.expr").(string),
	NotificationType: env.Get("EVENTS_WORKCHAT_NOTIFICATION_TYPE", "REGULAR").(string),
}

var grafanaOptions = render.GrafanaOptions{

	URL:         env.Get("EVENTS_GRAFANA_URL", "").(string),
	Timeout:     env.Get("EVENTS_GRAFANA_TIMEOUT", 60).(int),
	Datasource:  env.Get("EVENTS_GRAFANA_DATASOURCE", "Prometheus").(string),
	ApiKey:      env.Get("EVENTS_GRAFANA_API_KEY", "admin:admin").(string),
	Org:         env.Get("EVENTS_GRAFANA_ORG", "1").(string),
	Period:      env.Get("EVENTS_GRAFANA_PERIOD", 60).(int),
	ImageWidth:  env.Get("EVENTS_GRAFANA_IMAGE_WIDTH", 1280).(int),
	ImageHeight: env.Get("EVENTS_GRAFANA_IMAGE_HEIGHT", 640).(int),
}

var jaegerOptions = provider.JaegerOptions{
	ServiceName:         env.Get("EVENTS_JAEGER_SERVICE_NAME", "events").(string),
	AgentHost:           env.Get("EVENTS_JAEGER_AGENT_HOST", "").(string),
	AgentPort:           env.Get("EVENTS_JAEGER_AGENT_PORT", 6831).(int),
	Endpoint:            env.Get("EVENTS_JAEGER_ENDPOINT", "").(string),
	User:                env.Get("EVENTS_JAEGER_USER", "").(string),
	Password:            env.Get("EVENTS_JAEGER_PASSWORD", "").(string),
	BufferFlushInterval: env.Get("EVENTS_JAEGER_BUFFER_FLUSH_INTERVAL", 0).(int),
	QueueSize:           env.Get("EVENTS_JAEGER_QUEUE_SIZE", 0).(int),
	Tags:                env.Get("EVENTS_JAEGER_TAGS", "").(string),
}

var datadogOptions = provider.DataDogOptions{
	TracerHost:  env.Get("EVENTS_DATADOG_TRACER_HOST", "").(string),
	TracerPort:  env.Get("EVENTS_DATADOG_TRACER_PORT", 8126).(int),
	ServiceName: env.Get("EVENTS_DATADOG_SERVICE_NAME", "").(string),
	LoggerHost:  env.Get("EVENTS_DATADOG_TRACER_HOST", "").(string),
	LoggerPort:  env.Get("EVENTS_DATADOG_TRACER_PORT", 10518).(int),
}

func startMetrics(wg *sync.WaitGroup) {

	wg.Add(1)

	go func(wg *sync.WaitGroup) {

		defer wg.Done()

		logs.Info("Start metrics...")

		http.Handle(rootOptions.PrometheusURL, promhttp.Handler())

		listener, err := net.Listen("tcp", rootOptions.PrometheusListen)
		if err != nil {
			logs.Panic(err)
		}

		logs.Info("Metrics are up. Listening...")
		err = http.Serve(listener, nil)
		if err != nil {
			logs.Panic(err)
		}
	}(wg)
}

func interceptSyscall() {

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
	go func() {
		<-c
		logs.Info("Exiting...")
		os.Exit(1)
	}()
}

func Execute() {

	rootCmd := &cobra.Command{
		Use:   "events",
		Short: "Events",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			stdout = provider.NewStdout(stdoutOptions)
			stdout.SetCallerOffset(2)

			if common.HasElem(rootOptions.Logs, "stdout") {
				logs.Register(stdout)
			}

			jaeger := provider.NewJaeger(jaegerOptions, logs, stdout)
			jaeger.SetCallerOffset(1)

			if common.HasElem(rootOptions.Traces, "jaeger") {
				traces.Register(jaeger)
			}

			datadog := provider.NewDataDog(datadogOptions, logs, stdout)
			datadog.SetCallerOffset(1)

			if common.HasElem(rootOptions.Logs, "datadog") {
				logs.Register(datadog)
			}

			if common.HasElem(rootOptions.Traces, "datadog") {
				traces.Register(datadog)
			}

		},
		Run: func(cmd *cobra.Command, args []string) {

			var wg sync.WaitGroup

			logs.Info("Booting...")

			startMetrics(&wg)

			var httpInput common.Input = input.NewHttpInput(httpInputOptions, logs, traces)
			if reflect.ValueOf(httpInput).IsNil() {
				logs.Panic("Http input is invalid. Terminating...")
			}

			inputs := common.NewInputs()
			inputs.Add(&httpInput)

			outputs := common.NewOutputs(textTemplateOptions.TimeFormat, logs)

			var collectorOutput common.Output = output.NewCollectorOutput(&wg, collectorOutputOptions, textTemplateOptions, logs, traces)
			if reflect.ValueOf(collectorOutput).IsNil() {
				logs.Warn("Collector output is invalid. Skipping...")
			} else {
				outputs.Add(&collectorOutput)
			}

			var kafkaOutput common.Output = output.NewKafkaOutput(&wg, kafkaOutputOptions, textTemplateOptions, logs, traces)
			if reflect.ValueOf(kafkaOutput).IsNil() {
				logs.Warn("Kafka output is invalid. Skipping...")
			} else {
				outputs.Add(&kafkaOutput)
			}

			var telegramOutput common.Output = output.NewTelegramOutput(&wg, telegramOutputOptions, textTemplateOptions, grafanaOptions, logs, traces)
			if reflect.ValueOf(telegramOutput).IsNil() {
				logs.Warn("Telegram output is invalid. Skipping...")
			} else {
				outputs.Add(&telegramOutput)
			}

			var slackOutput common.Output = output.NewSlackOutput(&wg, slackOutputOptions, textTemplateOptions, grafanaOptions, logs, traces)
			if reflect.ValueOf(slackOutput).IsNil() {
				logs.Warn("Slack output is invalid. Skipping...")
			} else {
				outputs.Add(&slackOutput)
			}

			var workchatOutput common.Output = output.NewWorkchatOutput(&wg, workchatOutputOptions, textTemplateOptions, grafanaOptions, logs, traces)
			if reflect.ValueOf(workchatOutput).IsNil() {
				logs.Warn("Workchat output is invalid. Skipping...")
			} else {
				outputs.Add(&workchatOutput)
			}

			inputs.Start(&wg, outputs)
			wg.Wait()
		},
	}

	flags := rootCmd.PersistentFlags()

	flags.StringSliceVar(&rootOptions.Logs, "logs", rootOptions.Logs, "Log providers: stdout, datadog")
	flags.StringVar(&rootOptions.Metrics, "metrics", rootOptions.Metrics, "Metric providers")
	flags.StringSliceVar(&rootOptions.Traces, "traces", rootOptions.Traces, "Trace providers: jaeger, datadog")

	flags.StringVar(&rootOptions.PrometheusURL, "prometheus-url", rootOptions.PrometheusURL, "Prometheus endpoint url")
	flags.StringVar(&rootOptions.PrometheusListen, "prometheus-listen", rootOptions.PrometheusListen, "Prometheus listen")

	flags.StringVar(&textTemplateOptions.TimeFormat, "template-time-format", textTemplateOptions.TimeFormat, "Template time format")

	flags.StringVar(&stdoutOptions.Format, "stdout-format", stdoutOptions.Format, "Stdout format: json, text, template")
	flags.StringVar(&stdoutOptions.Level, "stdout-level", stdoutOptions.Level, "Stdout level: info, warn, error, debug, panic")
	flags.StringVar(&stdoutOptions.Template, "stdout-template", stdoutOptions.Template, "Stdout template")

	flags.StringVar(&httpInputOptions.K8sURL, "http-k8s-url", httpInputOptions.K8sURL, "Http K8s url")
	flags.StringVar(&httpInputOptions.RancherURL, "http-rancher-url", httpInputOptions.RancherURL, "Http Rancher url")
	flags.StringVar(&httpInputOptions.AlertmanagerURL, "http-alertmanager-url", httpInputOptions.AlertmanagerURL, "Http Alertmanager url")
	flags.StringVar(&httpInputOptions.Listen, "http-listen", httpInputOptions.Listen, "Http listen")
	flags.BoolVar(&httpInputOptions.Tls, "http-tls", httpInputOptions.Tls, "Http TLS")
	flags.StringVar(&httpInputOptions.Cert, "http-cert", httpInputOptions.Cert, "Http cert file or content")
	flags.StringVar(&httpInputOptions.Key, "http-key", httpInputOptions.Key, "Http key file or content")
	flags.StringVar(&httpInputOptions.Chain, "http-chain", httpInputOptions.Chain, "Http CA chain file or content")

	flags.StringVar(&kafkaOutputOptions.Brokers, "kafka-brokers", kafkaOutputOptions.Brokers, "Kafka brokers")
	flags.StringVar(&kafkaOutputOptions.Topic, "kafka-topic", kafkaOutputOptions.Topic, "Kafka topic")
	flags.StringVar(&kafkaOutputOptions.ClientID, "kafka-client-id", kafkaOutputOptions.ClientID, "Kafka client id")
	flags.StringVar(&kafkaOutputOptions.Template, "kafka-message-template", kafkaOutputOptions.Template, "Kafka message template")
	flags.IntVar(&kafkaOutputOptions.FlushFrequency, "kafka-flush-frequency", kafkaOutputOptions.FlushFrequency, "Kafka Producer flush frequency")
	flags.IntVar(&kafkaOutputOptions.FlushMaxMessages, "kafka-flush-max-messages", kafkaOutputOptions.FlushMaxMessages, "Kafka Producer flush max messages")
	flags.IntVar(&kafkaOutputOptions.NetMaxOpenRequests, "kafka-net-max-open-requests", kafkaOutputOptions.NetMaxOpenRequests, "Kafka Net max open requests")
	flags.IntVar(&kafkaOutputOptions.NetDialTimeout, "kafka-net-dial-timeout", kafkaOutputOptions.NetDialTimeout, "Kafka Net dial timeout")
	flags.IntVar(&kafkaOutputOptions.NetReadTimeout, "kafka-net-read-timeout", kafkaOutputOptions.NetReadTimeout, "Kafka Net read timeout")
	flags.IntVar(&kafkaOutputOptions.NetWriteTimeout, "kafka-net-write-timeout", kafkaOutputOptions.NetWriteTimeout, "Kafka Net write timeout")

	flags.StringVar(&telegramOutputOptions.URL, "telegram-url", telegramOutputOptions.URL, "Telegram URL")
	flags.StringVar(&telegramOutputOptions.MessageTemplate, "telegram-message-template", telegramOutputOptions.MessageTemplate, "Telegram message template")
	flags.StringVar(&telegramOutputOptions.SelectorTemplate, "telegram-selector-template", telegramOutputOptions.SelectorTemplate, "Telegram selector template")
	flags.IntVar(&telegramOutputOptions.Timeout, "telegram-timeout", telegramOutputOptions.Timeout, "Telegram timeout")
	flags.StringVar(&telegramOutputOptions.AlertExpression, "telegram-alert-expression", telegramOutputOptions.AlertExpression, "Telegram alert expression")
	flags.StringVar(&telegramOutputOptions.DisableNotification, "telegram-disable-notification", telegramOutputOptions.DisableNotification, "Telegram disable notification")

	flags.StringVar(&slackOutputOptions.URL, "slack-url", slackOutputOptions.URL, "Slack URL")
	flags.StringVar(&slackOutputOptions.MessageTemplate, "slack-message-template", slackOutputOptions.MessageTemplate, "Slack message template")
	flags.StringVar(&slackOutputOptions.SelectorTemplate, "slack-selector-template", slackOutputOptions.SelectorTemplate, "Slack selector template")
	flags.IntVar(&slackOutputOptions.Timeout, "slack-timeout", slackOutputOptions.Timeout, "Slack timeout")
	flags.StringVar(&slackOutputOptions.AlertExpression, "slack-alert-expression", slackOutputOptions.AlertExpression, "Slack alert expression")

	flags.StringVar(&workchatOutputOptions.URL, "workchat-url", workchatOutputOptions.URL, "Workchat URL")
	flags.StringVar(&workchatOutputOptions.MessageTemplate, "workchat-message-template", workchatOutputOptions.MessageTemplate, "Workchat message template")
	flags.StringVar(&workchatOutputOptions.SelectorTemplate, "workchat-selector-template", workchatOutputOptions.SelectorTemplate, "Workchat selector template")
	flags.IntVar(&workchatOutputOptions.Timeout, "workchat-timeout", workchatOutputOptions.Timeout, "Workchat timeout")
	flags.StringVar(&workchatOutputOptions.AlertExpression, "workchat-alert-expression", workchatOutputOptions.AlertExpression, "Workchat alert expression")
	flags.StringVar(&workchatOutputOptions.NotificationType, "workchat-notification-type", workchatOutputOptions.NotificationType, "Workchat notification type")

	flags.StringVar(&grafanaOptions.URL, "grafana-url", grafanaOptions.URL, "Grafana URL")
	flags.IntVar(&grafanaOptions.Timeout, "grafana-timeout", grafanaOptions.Timeout, "Grafan timeout")
	flags.StringVar(&grafanaOptions.Datasource, "grafana-datasource", grafanaOptions.Datasource, "Grafana datasource")
	flags.StringVar(&grafanaOptions.ApiKey, "grafana-api-key", grafanaOptions.ApiKey, "Grafana API key")
	flags.StringVar(&grafanaOptions.Org, "grafana-org", grafanaOptions.Org, "Grafana org")
	flags.IntVar(&grafanaOptions.Period, "grafana-period", grafanaOptions.Period, "Grafana period in minutes")
	flags.IntVar(&grafanaOptions.ImageWidth, "grafana-image-width", grafanaOptions.ImageWidth, "Grafan image width")
	flags.IntVar(&grafanaOptions.ImageHeight, "grafana-image-height", grafanaOptions.ImageHeight, "Grafan image height")

	flags.StringVar(&jaegerOptions.ServiceName, "jaeger-service-name", jaegerOptions.ServiceName, "Jaeger service name")
	flags.StringVar(&jaegerOptions.AgentHost, "jaeger-agent-host", jaegerOptions.AgentHost, "Jaeger agent host")
	flags.IntVar(&jaegerOptions.AgentPort, "jaeger-agent-port", jaegerOptions.AgentPort, "Jaeger agent port")
	flags.StringVar(&jaegerOptions.Endpoint, "jaeger-endpoint", jaegerOptions.Endpoint, "Jaeger endpoint")
	flags.StringVar(&jaegerOptions.User, "jaeger-user", jaegerOptions.User, "Jaeger user")
	flags.StringVar(&jaegerOptions.Password, "jaeger-password", jaegerOptions.Password, "Jaeger password")
	flags.IntVar(&jaegerOptions.BufferFlushInterval, "jaeger-buffer-flush-interval", jaegerOptions.BufferFlushInterval, "Jaeger buffer flush interval")
	flags.IntVar(&jaegerOptions.QueueSize, "jaeger-queue-size", jaegerOptions.QueueSize, "Jaeger queue size")
	flags.StringVar(&jaegerOptions.Tags, "jaeger-tags", jaegerOptions.Tags, "Jaeger tags, comma separated list of name=value")

	flags.StringVar(&datadogOptions.TracerHost, "datadog-tracer-host", datadogOptions.TracerHost, "DataDog tracer host")
	flags.IntVar(&datadogOptions.TracerPort, "datadog-tracer-port", datadogOptions.TracerPort, "Datadog tracer port")
	flags.StringVar(&datadogOptions.ServiceName, "datadog-service-name", datadogOptions.ServiceName, "DataDog service name")
	flags.StringVar(&datadogOptions.LoggerHost, "datadog-logger-host", datadogOptions.LoggerHost, "DataDog logger host")
	flags.IntVar(&datadogOptions.LoggerPort, "datadog-logger-port", datadogOptions.LoggerPort, "Datadog logger port")

	interceptSyscall()

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(VERSION)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		logs.Error(err)
		os.Exit(1)
	}
}
