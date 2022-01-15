package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"

	"sync"
	"syscall"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/input"
	"github.com/devopsext/events/output"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	utils "github.com/devopsext/utils"
	"github.com/spf13/cobra"
)

var VERSION = "unknown"
var APPNAME = "EVENTS"
var appName = strings.ToLower(APPNAME)

var env = utils.GetEnvironment()
var logs = sreCommon.NewLogs()
var traces = sreCommon.NewTraces()
var metrics = sreCommon.NewMetrics()
var events = sreCommon.NewEvents()
var stdout *sreProvider.Stdout
var mainWG sync.WaitGroup

type RootOptions struct {
	Logs    []string
	Metrics []string
	Traces  []string
	Events  []string
}

var rootOptions = RootOptions{
	Logs:    strings.Split(envGet("LOGS", "stdout").(string), ","),
	Metrics: strings.Split(envGet("METRICS", "prometheus").(string), ","),
	Traces:  strings.Split(envGet("TRACES", "").(string), ","),
	Events:  strings.Split(envGet("EVENTS", "").(string), ","),
}

var textTemplateOptions = render.TextTemplateOptions{
	TimeFormat: envGet("TEMPLATE_TIME_FORMAT", "2006-01-02T15:04:05.999Z").(string),
}

var stdoutOptions = sreProvider.StdoutOptions{
	Format:          envGet("STDOUT_FORMAT", "text").(string),
	Level:           envGet("STDOUT_LEVEL", "info").(string),
	Template:        envGet("STDOUT_TEMPLATE", "{{.file}} {{.msg}}").(string),
	TimestampFormat: envGet("STDOUT_TIMESTAMP_FORMAT", time.RFC3339Nano).(string),
	TextColors:      envGet("STDOUT_TEXT_COLORS", true).(bool),
}

var prometheusOptions = sreProvider.PrometheusOptions{

	URL:    envGet("PROMETHEUS_URL", "/metrics").(string),
	Listen: envGet("PROMETHEUS_LISTEN", "127.0.0.1:8080").(string),
	Prefix: envGet("PROMETHEUS_PREFIX", "events").(string),
}

var httpInputOptions = input.HttpInputOptions{

	K8sURL:          envGet("HTTP_K8S_URL", "").(string),
	RancherURL:      envGet("HTTP_RANCHER_URL", "").(string),
	AlertmanagerURL: envGet("HTTP_ALERTMANAGER_URL", "").(string),
	CustomJsonURL:   envGet("HTTP_CUSTOMJSON_URL", "").(string),
	Listen:          envGet("HTTP_LISTEN", ":80").(string),
	Tls:             envGet("HTTP_TLS", false).(bool),
	Cert:            envGet("HTTP_CERT", "").(string),
	Key:             envGet("HTTP_KEY", "").(string),
	Chain:           envGet("HTTP_CHAIN", "").(string),
	HeaderTraceID:   envGet("HTTP_HEADER_TRACE_ID", "X-Trace-ID").(string),
}

var collectorOutputOptions = output.CollectorOutputOptions{

	Address:  envGet("COLLECTOR_ADDRESS", "").(string),
	Template: envGet("COLLECTOR_MESSAGE_TEMPLATE", "").(string),
}

var kafkaOutputOptions = output.KafkaOutputOptions{

	ClientID:           envGet("KAFKA_CLIEND_ID", fmt.Sprintf("%s_kafka", appName)).(string),
	Template:           envGet("KAFKA_MESSAGE_TEMPLATE", "").(string),
	Brokers:            envGet("KAFKA_BROKERS", "").(string),
	Topic:              envGet("KAFKA_TOPIC", appName).(string),
	FlushFrequency:     envGet("KAFKA_FLUSH_FREQUENCY", 1).(int),
	FlushMaxMessages:   envGet("KAFKA_FLUSH_MAX_MESSAGES", 100).(int),
	NetMaxOpenRequests: envGet("KAFKA_NET_MAX_OPEN_REQUESTS", 5).(int),
	NetDialTimeout:     envGet("KAFKA_NET_DIAL_TIMEOUT", 30).(int),
	NetReadTimeout:     envGet("KAFKA_NET_READ_TIMEOUT", 30).(int),
	NetWriteTimeout:    envGet("KAFKA_NET_WRITE_TIMEOUT", 30).(int),
}

var telegramOutputOptions = output.TelegramOutputOptions{

	MessageTemplate:     envGet("TELEGRAM_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate:    envGet("TELEGRAM_SELECTOR_TEMPLATE", "").(string),
	URL:                 envGet("TELEGRAM_URL", "").(string),
	Timeout:             envGet("TELEGRAM_TIMEOUT", 30).(int),
	AlertExpression:     envGet("TELEGRAM_ALERT_EXPRESSION", "g0.expr").(string),
	DisableNotification: envGet("TELEGRAM_DISABLE_NOTIFICATION", "false").(string),
}

var slackOutputOptions = output.SlackOutputOptions{

	MessageTemplate:  envGet("SLACK_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate: envGet("SLACK_SELECTOR_TEMPLATE", "").(string),
	URL:              envGet("SLACK_URL", "").(string),
	Timeout:          envGet("SLACK_TIMEOUT", 30).(int),
	AlertExpression:  envGet("SLACK_ALERT_EXPRESSION", "g0.expr").(string),
}

var workchatOutputOptions = output.WorkchatOutputOptions{

	MessageTemplate:  envGet("WORKCHAT_MESSAGE_TEMPLATE", "").(string),
	SelectorTemplate: envGet("WORKCHAT_SELECTOR_TEMPLATE", "").(string),
	URL:              envGet("WORKCHAT_URL", "").(string),
	Timeout:          envGet("WORKCHAT_TIMEOUT", 30).(int),
	AlertExpression:  envGet("WORKCHAT_ALERT_EXPRESSION", "g0.expr").(string),
	NotificationType: envGet("WORKCHAT_NOTIFICATION_TYPE", "REGULAR").(string),
}

var grafanaOptions = render.GrafanaOptions{

	URL:         envGet("GRAFANA_URL", "").(string),
	Timeout:     envGet("GRAFANA_TIMEOUT", 60).(int),
	Datasource:  envGet("GRAFANA_DATASOURCE", "Prometheus").(string),
	ApiKey:      envGet("GRAFANA_API_KEY", "admin:admin").(string),
	Org:         envGet("GRAFANA_ORG", "1").(string),
	Period:      envGet("GRAFANA_PERIOD", 60).(int),
	ImageWidth:  envGet("GRAFANA_IMAGE_WIDTH", 1280).(int),
	ImageHeight: envGet("GRAFANA_IMAGE_HEIGHT", 640).(int),
}

var jaegerOptions = sreProvider.JaegerOptions{
	ServiceName:         envGet("JAEGER_SERVICE_NAME", appName).(string),
	AgentHost:           envGet("JAEGER_AGENT_HOST", "").(string),
	AgentPort:           envGet("JAEGER_AGENT_PORT", 6831).(int),
	Endpoint:            envGet("JAEGER_ENDPOINT", "").(string),
	User:                envGet("JAEGER_USER", "").(string),
	Password:            envGet("JAEGER_PASSWORD", "").(string),
	BufferFlushInterval: envGet("JAEGER_BUFFER_FLUSH_INTERVAL", 0).(int),
	QueueSize:           envGet("JAEGER_QUEUE_SIZE", 0).(int),
	Tags:                envGet("JAEGER_TAGS", "").(string),
	Debug:               envGet("JAEGER_DEBUG", false).(bool),
}

var datadogOptions = sreProvider.DataDogOptions{
	ServiceName: envGet("DATADOG_SERVICE_NAME", appName).(string),
	Environment: envGet("DATADOG_ENVIRONMENT", "none").(string),
	Tags:        envGet("DATADOG_TAGS", "").(string),
	Debug:       envGet("DATADOG_DEBUG", false).(bool),
}

var datadogTracerOptions = sreProvider.DataDogTracerOptions{
	AgentHost: envGet("DATADOG_TRACER_HOST", "").(string),
	AgentPort: envGet("DATADOG_TRACER_PORT", 8126).(int),
}

var datadogLoggerOptions = sreProvider.DataDogLoggerOptions{
	AgentHost: envGet("DATADOG_LOGGER_HOST", "").(string),
	AgentPort: envGet("DATADOG_LOGGER_PORT", 10518).(int),
	Level:     envGet("DATADOG_LOGGER_LEVEL", "info").(string),
}

var datadogMeterOptions = sreProvider.DataDogMeterOptions{
	AgentHost: envGet("DATADOG_METER_HOST", "").(string),
	AgentPort: envGet("DATADOG_METER_PORT", 10518).(int),
	Prefix:    envGet("DATADOG_METER_PREFIX", appName).(string),
}

var opentelemetryOptions = sreProvider.OpentelemetryOptions{
	ServiceName: envGet("OPENTELEMETRY_SERVICE_NAME", appName).(string),
	Environment: envGet("OPENTELEMETRY_ENVIRONMENT", "none").(string),
	Attributes:  envGet("OPENTELEMETRY_ATTRIBUTES", "").(string),
	Debug:       envGet("OPENTELEMETRY_DEBUG", false).(bool),
}

var opentelemetryTracerOptions = sreProvider.OpentelemetryTracerOptions{
	AgentHost: envGet("OPENTELEMETRY_TRACER_HOST", "").(string),
	AgentPort: envGet("OPENTELEMETRY_TRACER_PORT", 4317).(int),
}

var opentelemetryMeterOptions = sreProvider.OpentelemetryMeterOptions{
	AgentHost:     envGet("OPENTELEMETRY_METER_HOST", "").(string),
	AgentPort:     envGet("OPENTELEMETRY_METER_PORT", 4317).(int),
	Prefix:        envGet("OPENTELEMETRY_METER_PREFIX", appName).(string),
	CollectPeriod: int64(envGet("OPENTELEMETRY_METER_COLLECT_PERIOD", 1000).(int)),
}

func envGet(s string, d interface{}) interface{} {
	return env.Get(fmt.Sprintf("%s_%s", APPNAME, s), d)
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

			stdoutOptions.Version = VERSION
			stdout = sreProvider.NewStdout(stdoutOptions)
			if common.HasElem(rootOptions.Logs, "stdout") && stdout != nil {
				stdout.SetCallerOffset(2)
				logs.Register(stdout)
			}

			datadogLoggerOptions.Version = VERSION
			datadogLoggerOptions.ServiceName = datadogOptions.ServiceName
			datadogLoggerOptions.Environment = datadogOptions.Environment
			datadogLoggerOptions.Tags = datadogOptions.Tags
			datadogLoggerOptions.Debug = datadogOptions.Debug
			datadogLogger := sreProvider.NewDataDogLogger(datadogLoggerOptions, logs, stdout)
			if common.HasElem(rootOptions.Logs, "datadog") && datadogLogger != nil {
				logs.Register(datadogLogger)
			}

			logs.Info("Booting...")

			// Metrics

			prometheusOptions.Version = VERSION
			prometheus := sreProvider.NewPrometheusMeter(prometheusOptions, logs, stdout)
			if common.HasElem(rootOptions.Metrics, "prometheus") && prometheus != nil {
				prometheus.StartInWaitGroup(&mainWG)
				metrics.Register(prometheus)
			}

			datadogMeterOptions.Version = VERSION
			datadogMeterOptions.ServiceName = datadogOptions.ServiceName
			datadogMeterOptions.Environment = datadogOptions.Environment
			datadogMeterOptions.Tags = datadogOptions.Tags
			datadogMeterOptions.Debug = datadogOptions.Debug
			datadogMetricer := sreProvider.NewDataDogMeter(datadogMeterOptions, logs, stdout)
			if common.HasElem(rootOptions.Metrics, "datadog") && datadogMetricer != nil {
				metrics.Register(datadogMetricer)
			}

			opentelemetryMeterOptions.Version = VERSION
			opentelemetryMeterOptions.ServiceName = opentelemetryOptions.ServiceName
			opentelemetryMeterOptions.Environment = opentelemetryOptions.Environment
			opentelemetryMeterOptions.Attributes = opentelemetryOptions.Attributes
			opentelemetryMeterOptions.Debug = opentelemetryOptions.Debug
			opentelemetryMeter := sreProvider.NewOpentelemetryMeter(opentelemetryMeterOptions, logs, stdout)
			if common.HasElem(rootOptions.Metrics, "opentelemetry") && opentelemetryMeter != nil {
				metrics.Register(opentelemetryMeter)
			}

			// Tracing

			jaegerOptions.Version = VERSION
			jaeger := sreProvider.NewJaegerTracer(jaegerOptions, logs, stdout)
			if common.HasElem(rootOptions.Traces, "jaeger") && jaeger != nil {
				traces.Register(jaeger)
			}

			datadogTracerOptions.Version = VERSION
			datadogTracerOptions.ServiceName = datadogOptions.ServiceName
			datadogTracerOptions.Environment = datadogOptions.Environment
			datadogTracerOptions.Tags = datadogOptions.Tags
			datadogTracerOptions.Debug = datadogOptions.Debug
			datadogTracer := sreProvider.NewDataDogTracer(datadogTracerOptions, logs, stdout)
			if datadogTracer != nil {
				datadogTracer.SetCallerOffset(1)
				if common.HasElem(rootOptions.Traces, "datadog") {
					traces.Register(datadogTracer)
				}
			}

			opentelemetryTracerOptions.Version = VERSION
			opentelemetryTracerOptions.ServiceName = opentelemetryOptions.ServiceName
			opentelemetryTracerOptions.Environment = opentelemetryOptions.Environment
			opentelemetryTracerOptions.Attributes = opentelemetryOptions.Attributes
			opentelemetryTracerOptions.Debug = opentelemetryOptions.Debug
			opentelemtryTracer := sreProvider.NewOpentelemetryTracer(opentelemetryTracerOptions, logs, stdout)
			if common.HasElem(rootOptions.Traces, "opentelemetry") && opentelemtryTracer != nil {
				traces.Register(opentelemtryTracer)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {

			var httpInput common.Input = input.NewHttpInput(httpInputOptions, logs, traces, metrics, events)
			if reflect.ValueOf(httpInput).IsNil() {
				logs.Panic("Http input is invalid. Terminating...")
			}

			inputs := common.NewInputs()
			inputs.Add(&httpInput)

			outputs := common.NewOutputs(textTemplateOptions.TimeFormat, logs)

			var collectorOutput common.Output = output.NewCollectorOutput(&mainWG, collectorOutputOptions, textTemplateOptions, logs, traces, metrics)
			if reflect.ValueOf(collectorOutput).IsNil() {
				logs.Warn("Collector output is invalid. Skipping...")
			} else {
				outputs.Add(&collectorOutput)
			}

			var kafkaOutput common.Output = output.NewKafkaOutput(&mainWG, kafkaOutputOptions, textTemplateOptions, logs, traces, metrics)
			if reflect.ValueOf(kafkaOutput).IsNil() {
				logs.Warn("Kafka output is invalid. Skipping...")
			} else {
				outputs.Add(&kafkaOutput)
			}

			var telegramOutput common.Output = output.NewTelegramOutput(&mainWG, telegramOutputOptions, textTemplateOptions, grafanaOptions, logs, traces, metrics)
			if telegramOutput != nil {
				outputs.Add(&telegramOutput)
			} else {
				logs.Warn("Telegram output is invalid. Skipping...")
			}

			// if reflect.ValueOf(telegramOutput).IsNil() {
			// 	logs.Warn("Telegram output is invalid. Skipping...")
			// } else {
			// 	outputs.Add(&telegramOutput)
			// }

			var slackOutput common.Output = output.NewSlackOutput(&mainWG, slackOutputOptions, textTemplateOptions, grafanaOptions, logs, traces, metrics)
			if reflect.ValueOf(slackOutput).IsNil() {
				logs.Warn("Slack output is invalid. Skipping...")
			} else {
				outputs.Add(&slackOutput)
			}

			var workchatOutput common.Output = output.NewWorkchatOutput(&mainWG, workchatOutputOptions, textTemplateOptions, grafanaOptions, logs, traces, metrics)
			if reflect.ValueOf(workchatOutput).IsNil() {
				logs.Warn("Workchat output is invalid. Skipping...")
			} else {
				outputs.Add(&workchatOutput)
			}

			inputs.Start(&mainWG, outputs)
			mainWG.Wait()
		},
	}

	flags := rootCmd.PersistentFlags()

	flags.StringSliceVar(&rootOptions.Logs, "logs", rootOptions.Logs, "Log providers: stdout, datadog, newrelic")
	flags.StringSliceVar(&rootOptions.Metrics, "metrics", rootOptions.Metrics, "Metric providers: prometheus, datadog, opentelemetry, newrelic")
	flags.StringSliceVar(&rootOptions.Traces, "traces", rootOptions.Traces, "Trace providers: jaeger, datadog, opentelemetry, newrelic")
	flags.StringSliceVar(&rootOptions.Events, "events", rootOptions.Events, "Event providers: grafana, datadog, newrelic")

	flags.StringVar(&textTemplateOptions.TimeFormat, "template-time-format", textTemplateOptions.TimeFormat, "Template time format")

	flags.StringVar(&stdoutOptions.Format, "stdout-format", stdoutOptions.Format, "Stdout format: json, text, template")
	flags.StringVar(&stdoutOptions.Level, "stdout-level", stdoutOptions.Level, "Stdout level: info, warn, error, debug, panic")
	flags.StringVar(&stdoutOptions.Template, "stdout-template", stdoutOptions.Template, "Stdout template")
	flags.StringVar(&stdoutOptions.TimestampFormat, "stdout-timestamp-format", stdoutOptions.TimestampFormat, "Stdout timestamp format")
	flags.BoolVar(&stdoutOptions.TextColors, "stdout-text-colors", stdoutOptions.TextColors, "Stdout text colors")
	flags.BoolVar(&stdoutOptions.Debug, "stdout-debug", stdoutOptions.Debug, "Stdout debug")

	flags.StringVar(&prometheusOptions.URL, "prometheus-url", prometheusOptions.URL, "Prometheus endpoint url")
	flags.StringVar(&prometheusOptions.Listen, "prometheus-listen", prometheusOptions.Listen, "Prometheus listen")
	flags.StringVar(&prometheusOptions.Prefix, "prometheus-prefix", prometheusOptions.Prefix, "Prometheus prefix")

	flags.StringVar(&httpInputOptions.K8sURL, "http-k8s-url", httpInputOptions.K8sURL, "Http K8s url")
	flags.StringVar(&httpInputOptions.RancherURL, "http-rancher-url", httpInputOptions.RancherURL, "Http Rancher url")
	flags.StringVar(&httpInputOptions.AlertmanagerURL, "http-alertmanager-url", httpInputOptions.AlertmanagerURL, "Http Alertmanager url")
	flags.StringVar(&httpInputOptions.GitlabURL, "http-gitlab-url", httpInputOptions.GitlabURL, "Http Gitlab url")
	flags.StringVar(&httpInputOptions.CustomJsonURL, "http-customjson-url", httpInputOptions.CustomJsonURL, "Http CustomJson url")
	flags.StringVar(&httpInputOptions.Listen, "http-listen", httpInputOptions.Listen, "Http listen")
	flags.BoolVar(&httpInputOptions.Tls, "http-tls", httpInputOptions.Tls, "Http TLS")
	flags.StringVar(&httpInputOptions.Cert, "http-cert", httpInputOptions.Cert, "Http cert file or content")
	flags.StringVar(&httpInputOptions.Key, "http-key", httpInputOptions.Key, "Http key file or content")
	flags.StringVar(&httpInputOptions.Chain, "http-chain", httpInputOptions.Chain, "Http CA chain file or content")
	flags.StringVar(&httpInputOptions.HeaderTraceID, "http-header-trace-id", httpInputOptions.HeaderTraceID, "Http trace ID header")

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
	flags.BoolVar(&jaegerOptions.Debug, "jaeger-debug", jaegerOptions.Debug, "Jaeger debug")

	flags.StringVar(&datadogOptions.ServiceName, "datadog-service-name", datadogOptions.ServiceName, "DataDog service name")
	flags.StringVar(&datadogOptions.Environment, "datadog-environment", datadogOptions.Environment, "DataDog environment")
	flags.StringVar(&datadogOptions.Tags, "datadog-tags", datadogOptions.Tags, "DataDog tags")
	flags.BoolVar(&datadogOptions.Debug, "datadog-debug", datadogOptions.Debug, "DataDog debug")
	flags.StringVar(&datadogTracerOptions.AgentHost, "datadog-tracer-agent-host", datadogTracerOptions.AgentHost, "DataDog tracer agent host")
	flags.IntVar(&datadogTracerOptions.AgentPort, "datadog-tracer-agent-port", datadogTracerOptions.AgentPort, "Datadog tracer agent port")
	flags.StringVar(&datadogLoggerOptions.AgentHost, "datadog-logger-agent-host", datadogLoggerOptions.AgentHost, "DataDog logger agent host")
	flags.IntVar(&datadogLoggerOptions.AgentPort, "datadog-logger-agent-port", datadogLoggerOptions.AgentPort, "Datadog logger agent port")
	flags.StringVar(&datadogLoggerOptions.Level, "datadog-logger-level", datadogLoggerOptions.Level, "DataDog logger level: info, warn, error, debug, panic")
	flags.StringVar(&datadogMeterOptions.AgentHost, "datadog-meter-agent-host", datadogMeterOptions.AgentHost, "DataDog meter agent host")
	flags.IntVar(&datadogMeterOptions.AgentPort, "datadog-meter-agent-port", datadogMeterOptions.AgentPort, "Datadog meter agent port")
	flags.StringVar(&datadogMeterOptions.Prefix, "datadog-meter-prefix", datadogMeterOptions.Prefix, "DataDog meter prefix")

	flags.StringVar(&opentelemetryOptions.ServiceName, "opentelemetry-service-name", opentelemetryOptions.ServiceName, "Opentelemetry service name")
	flags.StringVar(&opentelemetryOptions.Environment, "opentelemetry-environment", opentelemetryOptions.Environment, "Opentelemetry environment")
	flags.StringVar(&opentelemetryOptions.Attributes, "opentelemetry-attributes", opentelemetryOptions.Attributes, "Opentelemetry attributes")
	flags.BoolVar(&opentelemetryOptions.Debug, "opentelemetry-debug", opentelemetryOptions.Debug, "Opentelemetry debug")
	flags.StringVar(&opentelemetryTracerOptions.AgentHost, "opentelemetry-tracer-agent-host", opentelemetryTracerOptions.AgentHost, "Opentelemetry tracer agent host")
	flags.IntVar(&opentelemetryTracerOptions.AgentPort, "opentelemetry-tracer-agent-port", opentelemetryTracerOptions.AgentPort, "Opentelemetry tracer agent port")
	flags.StringVar(&opentelemetryMeterOptions.AgentHost, "opentelemetry-meter-agent-host", opentelemetryMeterOptions.AgentHost, "Opentelemetry meter agent host")
	flags.IntVar(&opentelemetryMeterOptions.AgentPort, "opentelemetry-meter-agent-port", opentelemetryMeterOptions.AgentPort, "Opentelemetry meter agent port")
	flags.StringVar(&opentelemetryMeterOptions.Prefix, "opentelemetry-meter-prefix", opentelemetryMeterOptions.Prefix, "Opentelemetry meter prefix")

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
