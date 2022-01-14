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
}

var rootOptions = RootOptions{

	Logs:    strings.Split(env.Get(fmt.Sprintf("%s_LOGS", APPNAME), "stdout").(string), ","),
	Metrics: strings.Split(env.Get(fmt.Sprintf("%s_METRICS", APPNAME), "prometheus").(string), ","),
	Traces:  strings.Split(env.Get(fmt.Sprintf("%s_TRACES", APPNAME), "").(string), ","),
}

var textTemplateOptions = render.TextTemplateOptions{

	TimeFormat: env.Get(fmt.Sprintf("%s_TEMPLATE_TIME_FORMAT", APPNAME), "2006-01-02T15:04:05.999Z").(string),
}

var stdoutOptions = sreProvider.StdoutOptions{

	Format:          env.Get(fmt.Sprintf("%s_STDOUT_FORMAT", APPNAME), "text").(string),
	Level:           env.Get(fmt.Sprintf("%s_STDOUT_LEVEL", APPNAME), "info").(string),
	Template:        env.Get(fmt.Sprintf("%s_STDOUT_TEMPLATE", APPNAME), "{{.file}} {{.msg}}").(string),
	TimestampFormat: env.Get(fmt.Sprintf("%s_STDOUT_TIMESTAMP_FORMAT", APPNAME), time.RFC3339Nano).(string),
	TextColors:      env.Get(fmt.Sprintf("%s_STDOUT_TEXT_COLORS", APPNAME), true).(bool),
}

var prometheusOptions = sreProvider.PrometheusOptions{

	URL:    env.Get(fmt.Sprintf("%s_PROMETHEUS_URL", APPNAME), "/metrics").(string),
	Listen: env.Get(fmt.Sprintf("%s_PROMETHEUS_LISTEN", APPNAME), "127.0.0.1:8080").(string),
	Prefix: env.Get(fmt.Sprintf("%s_PROMETHEUS_PREFIX", APPNAME), "events").(string),
}

var httpInputOptions = input.HttpInputOptions{

	K8sURL:          env.Get(fmt.Sprintf("%s_HTTP_K8S_URL", APPNAME), "").(string),
	RancherURL:      env.Get(fmt.Sprintf("%s_HTTP_RANCHER_URL", APPNAME), "").(string),
	AlertmanagerURL: env.Get(fmt.Sprintf("%s_HTTP_ALERTMANAGER_URL", APPNAME), "").(string),
	CustomJsonURL:   env.Get(fmt.Sprintf("%s_HTTP_CUSTOMJSON_URL", APPNAME), "").(string),
	Listen:          env.Get(fmt.Sprintf("%s_HTTP_LISTEN", APPNAME), ":80").(string),
	Tls:             env.Get(fmt.Sprintf("%s_HTTP_TLS", APPNAME), false).(bool),
	Cert:            env.Get(fmt.Sprintf("%s_HTTP_CERT", APPNAME), "").(string),
	Key:             env.Get(fmt.Sprintf("%s_HTTP_KEY", APPNAME), "").(string),
	Chain:           env.Get(fmt.Sprintf("%s_HTTP_CHAIN", APPNAME), "").(string),
	HeaderTraceID:   env.Get(fmt.Sprintf("%s_HTTP_HEADER_TRACE_ID", APPNAME), "X-Trace-ID").(string),
}

var collectorOutputOptions = output.CollectorOutputOptions{

	Address:  env.Get(fmt.Sprintf("%s_COLLECTOR_ADDRESS", APPNAME), "").(string),
	Template: env.Get(fmt.Sprintf("%s_COLLECTOR_MESSAGE_TEMPLATE", APPNAME), "").(string),
}

var kafkaOutputOptions = output.KafkaOutputOptions{

	ClientID:           env.Get(fmt.Sprintf("%s_KAFKA_CLIEND_ID", APPNAME), fmt.Sprintf("%s_kafka", appName)).(string),
	Template:           env.Get(fmt.Sprintf("%s_KAFKA_MESSAGE_TEMPLATE", APPNAME), "").(string),
	Brokers:            env.Get(fmt.Sprintf("%s_KAFKA_BROKERS", APPNAME), "").(string),
	Topic:              env.Get(fmt.Sprintf("%s_KAFKA_TOPIC", APPNAME), appName).(string),
	FlushFrequency:     env.Get(fmt.Sprintf("%s_KAFKA_FLUSH_FREQUENCY", APPNAME), 1).(int),
	FlushMaxMessages:   env.Get(fmt.Sprintf("%s_KAFKA_FLUSH_MAX_MESSAGES", APPNAME), 100).(int),
	NetMaxOpenRequests: env.Get(fmt.Sprintf("%s_KAFKA_NET_MAX_OPEN_REQUESTS", APPNAME), 5).(int),
	NetDialTimeout:     env.Get(fmt.Sprintf("%s_KAFKA_NET_DIAL_TIMEOUT", APPNAME), 30).(int),
	NetReadTimeout:     env.Get(fmt.Sprintf("%s_KAFKA_NET_READ_TIMEOUT", APPNAME), 30).(int),
	NetWriteTimeout:    env.Get(fmt.Sprintf("%s_KAFKA_NET_WRITE_TIMEOUT", APPNAME), 30).(int),
}

var telegramOutputOptions = output.TelegramOutputOptions{

	MessageTemplate:     env.Get(fmt.Sprintf("%s_TELEGRAM_MESSAGE_TEMPLATE", APPNAME), "").(string),
	SelectorTemplate:    env.Get(fmt.Sprintf("%s_TELEGRAM_SELECTOR_TEMPLATE", APPNAME), "").(string),
	URL:                 env.Get(fmt.Sprintf("%s_TELEGRAM_URL", APPNAME), "").(string),
	Timeout:             env.Get(fmt.Sprintf("%s_TELEGRAM_TIMEOUT", APPNAME), 30).(int),
	AlertExpression:     env.Get(fmt.Sprintf("%s_TELEGRAM_ALERT_EXPRESSION", APPNAME), "g0.expr").(string),
	DisableNotification: env.Get(fmt.Sprintf("%s_TELEGRAM_DISABLE_NOTIFICATION", APPNAME), "false").(string),
}

var slackOutputOptions = output.SlackOutputOptions{

	MessageTemplate:  env.Get(fmt.Sprintf("%s_SLACK_MESSAGE_TEMPLATE", APPNAME), "").(string),
	SelectorTemplate: env.Get(fmt.Sprintf("%s_SLACK_SELECTOR_TEMPLATE", APPNAME), "").(string),
	URL:              env.Get(fmt.Sprintf("%s_SLACK_URL", APPNAME), "").(string),
	Timeout:          env.Get(fmt.Sprintf("%s_SLACK_TIMEOUT", APPNAME), 30).(int),
	AlertExpression:  env.Get(fmt.Sprintf("%s_SLACK_ALERT_EXPRESSION", APPNAME), "g0.expr").(string),
}

var workchatOutputOptions = output.WorkchatOutputOptions{

	MessageTemplate:  env.Get(fmt.Sprintf("%s_WORKCHAT_MESSAGE_TEMPLATE", APPNAME), "").(string),
	SelectorTemplate: env.Get(fmt.Sprintf("%s_WORKCHAT_SELECTOR_TEMPLATE", APPNAME), "").(string),
	URL:              env.Get(fmt.Sprintf("%s_WORKCHAT_URL", APPNAME), "").(string),
	Timeout:          env.Get(fmt.Sprintf("%s_WORKCHAT_TIMEOUT", APPNAME), 30).(int),
	AlertExpression:  env.Get(fmt.Sprintf("%s_WORKCHAT_ALERT_EXPRESSION", APPNAME), "g0.expr").(string),
	NotificationType: env.Get(fmt.Sprintf("%s_WORKCHAT_NOTIFICATION_TYPE", APPNAME), "REGULAR").(string),
}

var grafanaOptions = render.GrafanaOptions{

	URL:         env.Get(fmt.Sprintf("%s_GRAFANA_URL", APPNAME), "").(string),
	Timeout:     env.Get(fmt.Sprintf("%s_GRAFANA_TIMEOUT", APPNAME), 60).(int),
	Datasource:  env.Get(fmt.Sprintf("%s_GRAFANA_DATASOURCE", APPNAME), "Prometheus").(string),
	ApiKey:      env.Get(fmt.Sprintf("%s_GRAFANA_API_KEY", APPNAME), "admin:admin").(string),
	Org:         env.Get(fmt.Sprintf("%s_GRAFANA_ORG", APPNAME), "1").(string),
	Period:      env.Get(fmt.Sprintf("%s_GRAFANA_PERIOD", APPNAME), 60).(int),
	ImageWidth:  env.Get(fmt.Sprintf("%s_GRAFANA_IMAGE_WIDTH", APPNAME), 1280).(int),
	ImageHeight: env.Get(fmt.Sprintf("%s_GRAFANA_IMAGE_HEIGHT", APPNAME), 640).(int),
}

var jaegerOptions = sreProvider.JaegerOptions{
	ServiceName:         env.Get(fmt.Sprintf("%s_JAEGER_SERVICE_NAME", APPNAME), appName).(string),
	AgentHost:           env.Get(fmt.Sprintf("%s_JAEGER_AGENT_HOST", APPNAME), "").(string),
	AgentPort:           env.Get(fmt.Sprintf("%s_JAEGER_AGENT_PORT", APPNAME), 6831).(int),
	Endpoint:            env.Get(fmt.Sprintf("%s_JAEGER_ENDPOINT", APPNAME), "").(string),
	User:                env.Get(fmt.Sprintf("%s_JAEGER_USER", APPNAME), "").(string),
	Password:            env.Get(fmt.Sprintf("%s_JAEGER_PASSWORD", APPNAME), "").(string),
	BufferFlushInterval: env.Get(fmt.Sprintf("%s_JAEGER_BUFFER_FLUSH_INTERVAL", APPNAME), 0).(int),
	QueueSize:           env.Get(fmt.Sprintf("%s_JAEGER_QUEUE_SIZE", APPNAME), 0).(int),
	Tags:                env.Get(fmt.Sprintf("%s_JAEGER_TAGS", APPNAME), "").(string),
	Debug:               env.Get(fmt.Sprintf("%s_JAEGER_DEBUG", APPNAME), false).(bool),
}

var datadogOptions = sreProvider.DataDogOptions{
	ServiceName: env.Get(fmt.Sprintf("%s_DATADOG_SERVICE_NAME", APPNAME), appName).(string),
	Environment: env.Get(fmt.Sprintf("%s_DATADOG_ENVIRONMENT", APPNAME), "none").(string),
	Tags:        env.Get(fmt.Sprintf("%s_DATADOG_TAGS", APPNAME), "").(string),
	Debug:       env.Get(fmt.Sprintf("%s_DATADOG_DEBUG", APPNAME), false).(bool),
}

var datadogTracerOptions = sreProvider.DataDogTracerOptions{
	AgentHost: env.Get(fmt.Sprintf("%s_DATADOG_TRACER_HOST", APPNAME), "").(string),
	AgentPort: env.Get(fmt.Sprintf("%s_DATADOG_TRACER_PORT", APPNAME), 8126).(int),
}

var datadogLoggerOptions = sreProvider.DataDogLoggerOptions{
	AgentHost: env.Get(fmt.Sprintf("%s_DATADOG_LOGGER_HOST", APPNAME), "").(string),
	AgentPort: env.Get(fmt.Sprintf("%s_DATADOG_LOGGER_PORT", APPNAME), 10518).(int),
	Level:     env.Get(fmt.Sprintf("%s_DATADOG_LOGGER_LEVEL", APPNAME), "info").(string),
}

var datadogMeterOptions = sreProvider.DataDogMeterOptions{
	AgentHost: env.Get(fmt.Sprintf("%s_DATADOG_METER_HOST", APPNAME), "").(string),
	AgentPort: env.Get(fmt.Sprintf("%s_DATADOG_METER_PORT", APPNAME), 10518).(int),
	Prefix:    env.Get(fmt.Sprintf("%s_DATADOG_METER_PREFIX", APPNAME), appName).(string),
}

var opentelemetryOptions = sreProvider.OpentelemetryOptions{
	ServiceName: env.Get(fmt.Sprintf("%s_OPENTELEMETRY_SERVICE_NAME", APPNAME), appName).(string),
	Environment: env.Get(fmt.Sprintf("%s_OPENTELEMETRY_ENVIRONMENT", APPNAME), "none").(string),
	Attributes:  env.Get(fmt.Sprintf("%s_OPENTELEMETRY_ATTRIBUTES", APPNAME), "").(string),
	Debug:       env.Get(fmt.Sprintf("%s_OPENTELEMETRY_DEBUG", APPNAME), false).(bool),
}

var opentelemetryTracerOptions = sreProvider.OpentelemetryTracerOptions{
	AgentHost: env.Get(fmt.Sprintf("%s_OPENTELEMETRY_TRACER_HOST", APPNAME), "").(string),
	AgentPort: env.Get(fmt.Sprintf("%s_OPENTELEMETRY_TRACER_PORT", APPNAME), 4317).(int),
}

var opentelemetryMeterOptions = sreProvider.OpentelemetryMeterOptions{
	AgentHost:     env.Get(fmt.Sprintf("%s_OPENTELEMETRY_METER_HOST", APPNAME), "").(string),
	AgentPort:     env.Get(fmt.Sprintf("%s_OPENTELEMETRY_METER_PORT", APPNAME), 4317).(int),
	Prefix:        env.Get(fmt.Sprintf("%s_OPENTELEMETRY_METER_PREFIX", APPNAME), appName).(string),
	CollectPeriod: int64(env.Get(fmt.Sprintf("%s_OPENTELEMETRY_METER_COLLECT_PERIOD", APPNAME), 1000).(int)),
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
			if reflect.ValueOf(telegramOutput).IsNil() {
				logs.Warn("Telegram output is invalid. Skipping...")
			} else {
				outputs.Add(&telegramOutput)
			}

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

	flags.StringSliceVar(&rootOptions.Logs, "logs", rootOptions.Logs, "Log providers: stdout, datadog")
	flags.StringSliceVar(&rootOptions.Metrics, "metrics", rootOptions.Metrics, "Metric providers: prometheus, datadog, opentelemetry")
	flags.StringSliceVar(&rootOptions.Traces, "traces", rootOptions.Traces, "Trace providers: jaeger, datadog, opentelemetry")

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
