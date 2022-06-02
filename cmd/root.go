package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"sync"
	"syscall"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/input"
	"github.com/devopsext/events/output"
	"github.com/devopsext/events/processor"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	sreProvider "github.com/devopsext/sre/provider"
	"github.com/devopsext/tools/vendors"
	"github.com/devopsext/utils"
	"github.com/spf13/cobra"
)

var version = "unknown"
var APPNAME = "EVENTS"
var appName = strings.ToLower(APPNAME)

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
	HealthcheckURL:  envGet("HTTP_IN_HEALTHCHECK_URL", "/healthcheck").(string),
	K8sURL:          envGet("HTTP_IN_K8S_URL", "").(string),
	RancherURL:      envGet("HTTP_IN_RANCHER_URL", "").(string),
	AlertmanagerURL: envGet("HTTP_IN_ALERTMANAGER_URL", "").(string),
	GitlabURL:       envGet("HTTP_IN_GITLAB_URL", "").(string),
	DataDogURL:      envGet("HTTP_IN_DATADOG_URL", "").(string),
	CustomJsonURL:   envGet("HTTP_IN_CUSTOMJSON_URL", "").(string),
	AWSURL:          envGet("HTTP_IN_AWS_URL", "").(string),
	GoogleURL:       envGet("HTTP_IN_GOOGLE_URL", "").(string),
	CloudflareURL:   envGet("HTTP_IN_CLOUDFLARE_URL", "").(string),
	Site24x7URL:     envGet("HTTP_IN_SITE24X7_URL", "").(string),
	Listen:          envGet("HTTP_IN_LISTEN", ":80").(string),
	Tls:             envGet("HTTP_IN_TLS", false).(bool),
	Cert:            envGet("HTTP_IN_CERT", "").(string),
	Key:             envGet("HTTP_IN_KEY", "").(string),
	Chain:           envGet("HTTP_IN_CHAIN", "").(string),
	HeaderTraceID:   envGet("HTTP_IN_HEADER_TRACE_ID", "X-Trace-ID").(string),
}

var pubsubInputOptions = input.PubSubInputOptions{
	Credentials:  envGet("PUBSUB_IN_CREDENTIALS", "").(string),
	ProjectID:    envGet("PUBSUB_IN_PROJECT_ID", "").(string),
	Subscription: envGet("PUBSUB_IN_SUBSCRIPTION", "").(string),
}

var collectorOutputOptions = output.CollectorOutputOptions{
	Address: envGet("COLLECTOR_OUT_ADDRESS", "").(string),
	Message: envGet("COLLECTOR_OUT_MESSAGE", "").(string),
}

var kafkaOutputOptions = output.KafkaOutputOptions{
	ClientID:           envGet("KAFKA_OUT_CLIEND_ID", fmt.Sprintf("%s_kafka", appName)).(string),
	Message:            envGet("KAFKA_OUT_MESSAGE", "").(string),
	Brokers:            envGet("KAFKA_OUT_BROKERS", "").(string),
	Topic:              envGet("KAFKA_OUT_TOPIC", appName).(string),
	FlushFrequency:     envGet("KAFKA_OUT_FLUSH_FREQUENCY", 1).(int),
	FlushMaxMessages:   envGet("KAFKA_OUT_FLUSH_MAX_MESSAGES", 100).(int),
	NetMaxOpenRequests: envGet("KAFKA_OUT_NET_MAX_OPEN_REQUESTS", 5).(int),
	NetDialTimeout:     envGet("KAFKA_OUT_NET_DIAL_TIMEOUT", 30).(int),
	NetReadTimeout:     envGet("KAFKA_OUT_NET_READ_TIMEOUT", 30).(int),
	NetWriteTimeout:    envGet("KAFKA_OUT_NET_WRITE_TIMEOUT", 30).(int),
}

var telegramOutputOptions = output.TelegramOutputOptions{
	TelegramOptions: vendors.TelegramOptions{
		IDToken:             envGet("TELEGRAM_OUT_ID_TOKEN", "").(string),
		ChatID:              envGet("TELEGRAM_OUT_CHAT_ID", "").(string),
		Timeout:             envGet("TELEGRAM_OUT_TIMEOUT", 30).(int),
		ParseMode:           envGet("TELEGRAM_OUT_PARSE_MODE", "HTML").(string),
		DisableNotification: envGet("TELEGRAM_OUT_DISABLE_NOTIFICATION", false).(bool),
	},
	Message:         envGet("TELEGRAM_OUT_MESSAGE", "").(string),
	BotSelector:     envGet("TELEGRAM_OUT_BOT_SELECTOR", "").(string),
	AlertExpression: envGet("TELEGRAM_OUT_ALERT_EXPRESSION", "g0.expr").(string),
	Forward:         envGet("TELEGRAM_OUT_FORWARD", "").(string),
	RateLimit:       envGet("TELEGRAM_RATELIMIT", 15).(int),
}

var slackOutputOptions = output.SlackOutputOptions{
	ChannelSelector: envGet("SLACK_OUT_CHANNEL_SELECTOR", "").(string),
	Timeout:         envGet("SLACK_OUT_TIMEOUT", 30).(int),
	Message:         envGet("SLACK_OUT_MESSAGE", "").(string),
	AlertExpression: envGet("SLACK_OUT_ALERT_EXPRESSION", "g0.expr").(string),
	Token:           envGet("SLACK_OUT_TOKEN", "").(string),
	Channel:         envGet("SLACK_OUT_CHANNEL", "").(string),
	Forward:         envGet("SLACK_OUT_FORWARD", "").(string),
}

var workchatOutputOptions = output.WorkchatOutputOptions{
	Message:          envGet("WORKCHAT_OUT_MESSAGE", "").(string),
	URLSelector:      envGet("WORKCHAT_OUT_URL_SELECTOR", "").(string),
	URL:              envGet("WORKCHAT_OUT_URL", "").(string),
	Timeout:          envGet("WORKCHAT_OUT_TIMEOUT", 30).(int),
	AlertExpression:  envGet("WORKCHAT_OUT_ALERT_EXPRESSION", "g0.expr").(string),
	NotificationType: envGet("WORKCHAT_OUT_NOTIFICATION_TYPE", "REGULAR").(string),
}

var newrelicOutputOptions = output.NewRelicOutputOptions{
	Message:            envGet("NEWRELIC_OUT_MESSAGE", "").(string),
	AttributesSelector: envGet("NEWRELIC_OUT_ATTRIBUTES_SELECTOR", "").(string),
}

var datadogOutputOptions = output.DataDogOutputOptions{
	Message:            envGet("DATADOG_OUT_MESSAGE", "").(string),
	AttributesSelector: envGet("DATADOG_OUT_ATTRIBUTES_SELECTOR", "").(string),
}

var grafanaOutputOptions = output.GrafanaOutputOptions{
	Message:            envGet("GRAFANA_OUT_MESSAGE", "").(string),
	AttributesSelector: envGet("GRAFANA_OUT_ATTRIBUTES_SELECTOR", "").(string),
}

var pubsubOutputOptions = output.PubSubOutputOptions{
	Credentials:   envGet("PUBSUB_OUT_CREDENTIALS", "").(string),
	ProjectID:     envGet("PUBSUB_OUT_PROJECT_ID", "").(string),
	Message:       envGet("PUBSUB_OUT_MESSAGE", "").(string),
	TopicSelector: envGet("PUBSUB_OUT_TOPIC_SELECTOR", "").(string),
}

var gitlabOutputOptions = output.GitlabOutputOptions{
	BaseURL:   envGet("GITLAB_OUT_BASE_URL", "").(string),
	Token:     envGet("GITLAB_OUT_TOKEN", "").(string),
	Projects:  envGet("GITLAB_OUT_PROJECTS", "").(string),
	Variables: envGet("GITLAB_OUT_VARIABLES", "").(string),
}

var grafanaRenderOptions = render.GrafanaRenderOptions{
	URL:         envGet("GRAFANA_RENDER_URL", "").(string),
	Timeout:     envGet("GRAFANA_RENDER_TIMEOUT", 60).(int),
	Datasource:  envGet("GRAFANA_RENDER_DATASOURCE", "Prometheus").(string),
	ApiKey:      envGet("GRAFANA_RENDER_API_KEY", "admin:admin").(string),
	Org:         envGet("GRAFANA_RENDER_ORG", "1").(string),
	Period:      envGet("GRAFANA_RENDER_PERIOD", 60).(int),
	ImageWidth:  envGet("GRAFANA_RENDER_IMAGE_WIDTH", 1280).(int),
	ImageHeight: envGet("GRAFANA_RENDER_IMAGE_HEIGHT", 640).(int),
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
	ApiKey:      envGet("DATADOG_API_KEY", "").(string),
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

var datadogEventerOptions = sreProvider.DataDogEventerOptions{
	Site: envGet("DATADOG_EVENTER_SITE", "").(string),
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

var newrelicOptions = sreProvider.NewRelicOptions{
	ApiKey:      envGet("NEWRELIC_API_KEY", "").(string),
	ServiceName: envGet("NEWRELIC_SERVICE_NAME", appName).(string),
	Environment: envGet("NEWRELIC_ENVIRONMENT", "").(string),
	Attributes:  envGet("NEWRELIC_ATTRIBUTES", "").(string),
	Debug:       envGet("NEWRELIC_DEBUG", false).(bool),
}

var newrelicTracerOptions = sreProvider.NewRelicTracerOptions{
	Endpoint: envGet("NEWRELIC_TRACER_ENDPOINT", "").(string),
}

var newrelicLoggerOptions = sreProvider.NewRelicLoggerOptions{
	Endpoint:  envGet("NEWRELIC_LOGGER_ENDPOINT", "").(string),
	AgentHost: envGet("NEWRELIC_LOGGER_HOST", "").(string),
	AgentPort: envGet("NEWRELIC_LOGGER_PORT", 5171).(int),
	Level:     envGet("NEWRELIC_LOGGER_LEVEL", "info").(string),
}

var newrelicMeterOptions = sreProvider.NewRelicMeterOptions{
	Endpoint: envGet("NEWRELIC_METER_ENDPOINT", "").(string),
	Prefix:   envGet("NEWRELIC_METER_PREFIX", appName).(string),
}

var newrelicEventerOptions = sreProvider.NewRelicEventerOptions{
	Endpoint: envGet("NEWRELIC_EVENTER_ENDPOINT", "").(string),
}

var grafanaOptions = sreProvider.GrafanaOptions{
	URL:     envGet("GRAFANA_URL", "").(string),
	ApiKey:  envGet("GRAFANA_API_KEY", "admin:admin").(string),
	Tags:    envGet("GRAFANA_TAGS", "").(string),
	Timeout: envGet("GRAFANA_TIMEOUT", 60).(int),
}

var grafanaEventerOptions = sreProvider.GrafanaEventerOptions{
	Endpoint: envGet("GRAFANA_EVENTER_ENDPOINT", "").(string),
	Duration: envGet("GRAFANA_EVENTER_DURATION", 1).(int),
}

func envGet(s string, d interface{}) interface{} {
	return utils.EnvGet(fmt.Sprintf("%s_%s", APPNAME, s), d)
}

func interceptSyscall() {

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		logs.Info("Exiting...")
		os.Exit(1)
	}()
}

func Execute() {

	var newrelicEventer *sreProvider.NewRelicEventer
	var grafanaEventer *sreProvider.GrafanaEventer
	var datadogEventer *sreProvider.DataDogEventer

	rootCmd := &cobra.Command{
		Use:   "events",
		Short: "Events",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			stdoutOptions.Version = version
			stdout = sreProvider.NewStdout(stdoutOptions)
			if utils.Contains(rootOptions.Logs, "stdout") && stdout != nil {
				stdout.SetCallerOffset(2)
				logs.Register(stdout)
			}

			datadogLoggerOptions.Version = version
			datadogLoggerOptions.ServiceName = datadogOptions.ServiceName
			datadogLoggerOptions.Environment = datadogOptions.Environment
			datadogLoggerOptions.Tags = datadogOptions.Tags
			datadogLoggerOptions.Debug = datadogOptions.Debug
			datadogLogger := sreProvider.NewDataDogLogger(datadogLoggerOptions, logs, stdout)
			if utils.Contains(rootOptions.Logs, "datadog") && datadogLogger != nil {
				logs.Register(datadogLogger)
			}

			newrelicLoggerOptions.Version = version
			newrelicLoggerOptions.ApiKey = newrelicOptions.ApiKey
			newrelicLoggerOptions.ServiceName = newrelicOptions.ServiceName
			newrelicLoggerOptions.Environment = newrelicOptions.Environment
			newrelicLoggerOptions.Attributes = newrelicOptions.Attributes
			newrelicLoggerOptions.Debug = newrelicOptions.Debug
			newrelicLogger := sreProvider.NewNewRelicLogger(newrelicLoggerOptions, logs, stdout)
			if utils.Contains(rootOptions.Logs, "newrelic") && newrelicLogger != nil {
				logs.Register(newrelicLogger)
			}

			logs.Info("Booting...")

			// Metrics

			prometheusOptions.Version = version
			prometheus := sreProvider.NewPrometheusMeter(prometheusOptions, logs, stdout)
			if utils.Contains(rootOptions.Metrics, "prometheus") && prometheus != nil {
				prometheus.StartInWaitGroup(&mainWG)
				metrics.Register(prometheus)
			}

			datadogMeterOptions.Version = version
			datadogMeterOptions.ServiceName = datadogOptions.ServiceName
			datadogMeterOptions.Environment = datadogOptions.Environment
			datadogMeterOptions.Tags = datadogOptions.Tags
			datadogMeterOptions.Debug = datadogOptions.Debug
			datadogMetricer := sreProvider.NewDataDogMeter(datadogMeterOptions, logs, stdout)
			if utils.Contains(rootOptions.Metrics, "datadog") && datadogMetricer != nil {
				metrics.Register(datadogMetricer)
			}

			opentelemetryMeterOptions.Version = version
			opentelemetryMeterOptions.ServiceName = opentelemetryOptions.ServiceName
			opentelemetryMeterOptions.Environment = opentelemetryOptions.Environment
			opentelemetryMeterOptions.Attributes = opentelemetryOptions.Attributes
			opentelemetryMeterOptions.Debug = opentelemetryOptions.Debug
			opentelemetryMeter := sreProvider.NewOpentelemetryMeter(opentelemetryMeterOptions, logs, stdout)
			if utils.Contains(rootOptions.Metrics, "opentelemetry") && opentelemetryMeter != nil {
				metrics.Register(opentelemetryMeter)
			}

			newrelicMeterOptions.Version = version
			newrelicMeterOptions.ApiKey = newrelicOptions.ApiKey
			newrelicMeterOptions.ServiceName = newrelicOptions.ServiceName
			newrelicMeterOptions.Environment = newrelicOptions.Environment
			newrelicMeterOptions.Attributes = newrelicOptions.Attributes
			newrelicMeterOptions.Debug = newrelicOptions.Debug
			newrelicMeter := sreProvider.NewNewRelicMeter(newrelicMeterOptions, logs, stdout)
			if utils.Contains(rootOptions.Metrics, "newrelic") && newrelicMeter != nil {
				metrics.Register(newrelicMeter)
			}

			// Tracing

			jaegerOptions.Version = version
			jaeger := sreProvider.NewJaegerTracer(jaegerOptions, logs, stdout)
			if utils.Contains(rootOptions.Traces, "jaeger") && jaeger != nil {
				traces.Register(jaeger)
			}

			datadogTracerOptions.Version = version
			datadogTracerOptions.ServiceName = datadogOptions.ServiceName
			datadogTracerOptions.Environment = datadogOptions.Environment
			datadogTracerOptions.Tags = datadogOptions.Tags
			datadogTracerOptions.Debug = datadogOptions.Debug
			datadogTracer := sreProvider.NewDataDogTracer(datadogTracerOptions, logs, stdout)
			if datadogTracer != nil {
				datadogTracer.SetCallerOffset(1)
				if utils.Contains(rootOptions.Traces, "datadog") {
					traces.Register(datadogTracer)
				}
			}

			opentelemetryTracerOptions.Version = version
			opentelemetryTracerOptions.ServiceName = opentelemetryOptions.ServiceName
			opentelemetryTracerOptions.Environment = opentelemetryOptions.Environment
			opentelemetryTracerOptions.Attributes = opentelemetryOptions.Attributes
			opentelemetryTracerOptions.Debug = opentelemetryOptions.Debug
			opentelemtryTracer := sreProvider.NewOpentelemetryTracer(opentelemetryTracerOptions, logs, stdout)
			if utils.Contains(rootOptions.Traces, "opentelemetry") && opentelemtryTracer != nil {
				traces.Register(opentelemtryTracer)
			}

			newrelicTracerOptions.Version = version
			newrelicTracerOptions.ApiKey = newrelicOptions.ApiKey
			newrelicTracerOptions.ServiceName = newrelicOptions.ServiceName
			newrelicTracerOptions.Environment = newrelicOptions.Environment
			newrelicTracerOptions.Attributes = newrelicOptions.Attributes
			newrelicTracerOptions.Debug = newrelicOptions.Debug
			newrelicTracer := sreProvider.NewNewRelicTracer(newrelicTracerOptions, logs, stdout)
			if utils.Contains(rootOptions.Metrics, "newrelic") && newrelicTracer != nil {
				traces.Register(newrelicTracer)
			}

			// Events

			newrelicEventerOptions.Version = version
			newrelicEventerOptions.ApiKey = newrelicOptions.ApiKey
			newrelicEventerOptions.ServiceName = newrelicOptions.ServiceName
			newrelicEventerOptions.Environment = newrelicOptions.Environment
			newrelicEventerOptions.Attributes = newrelicOptions.Attributes
			newrelicEventerOptions.Debug = newrelicOptions.Debug
			newrelicEventer = sreProvider.NewNewRelicEventer(newrelicEventerOptions, logs, stdout)
			if utils.Contains(rootOptions.Events, "newrelic") && newrelicEventer != nil {
				events.Register(newrelicEventer)
			}

			datadogEventerOptions.Version = version
			datadogEventerOptions.ApiKey = datadogOptions.ApiKey
			datadogEventerOptions.ServiceName = datadogOptions.ServiceName
			datadogEventerOptions.Environment = datadogOptions.Environment
			datadogEventerOptions.Tags = datadogOptions.Tags
			datadogEventerOptions.Debug = datadogOptions.Debug
			datadogEventer = sreProvider.NewDataDogEventer(datadogEventerOptions, logs, stdout)
			if utils.Contains(rootOptions.Events, "datadog") && datadogEventer != nil {
				events.Register(datadogEventer)
			}

			grafanaEventerOptions.Version = version
			grafanaEventerOptions.URL = grafanaOptions.URL
			grafanaEventerOptions.ApiKey = grafanaOptions.ApiKey
			grafanaEventerOptions.Tags = grafanaOptions.Tags
			grafanaEventerOptions.Timeout = grafanaOptions.Timeout
			grafanaEventer = sreProvider.NewGrafanaEventer(grafanaEventerOptions, logs, stdout)
			if utils.Contains(rootOptions.Events, "grafana") && grafanaEventer != nil {
				events.Register(grafanaEventer)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {

			observability := common.NewObservability(logs, traces, metrics, events)
			outputs := common.NewOutputs(logs)

			processors := common.NewProcessors()
			processors.Add(processor.NewK8sProcessor(&outputs, observability))
			processors.Add(processor.NewGitlabProcessor(&outputs, observability))
			processors.Add(processor.NewAlertmanagerProcessor(&outputs, observability))
			processors.Add(processor.NewCustomJsonProcessor(&outputs, observability))
			processors.Add(processor.NewRancherProcessor(&outputs, observability))
			processors.Add(processor.NewDataDogProcessor(&outputs, observability))
			processors.Add(processor.NewSite24x7Processor(&outputs, observability))
			processors.Add(processor.NewCloudflareProcessor(&outputs, observability))
			processors.Add(processor.NewGoogleProcessor(&outputs, observability))
			processors.Add(processor.NewAWSProcessor(&outputs, observability))

			inputs := common.NewInputs()
			inputs.Add(input.NewHttpInput(httpInputOptions, processors, observability))
			inputs.Add(input.NewPubSubInput(pubsubInputOptions, processors, observability))

			outputs.Add(output.NewCollectorOutput(&mainWG, collectorOutputOptions, textTemplateOptions, observability))
			outputs.Add(output.NewKafkaOutput(&mainWG, kafkaOutputOptions, textTemplateOptions, observability))
			outputs.Add(output.NewTelegramOutput(&mainWG, telegramOutputOptions, textTemplateOptions, grafanaRenderOptions, observability, &outputs))
			outputs.Add(output.NewSlackOutput(&mainWG, slackOutputOptions, textTemplateOptions, grafanaRenderOptions, observability, &outputs))
			outputs.Add(output.NewWorkchatOutput(&mainWG, workchatOutputOptions, textTemplateOptions, grafanaRenderOptions, observability))
			outputs.Add(output.NewNewRelicOutput(&mainWG, newrelicOutputOptions, textTemplateOptions, observability, newrelicEventer))
			outputs.Add(output.NewDataDogOutput(&mainWG, datadogOutputOptions, textTemplateOptions, observability, datadogEventer))
			outputs.Add(output.NewGrafanaOutput(&mainWG, grafanaOutputOptions, textTemplateOptions, observability, grafanaEventer))
			outputs.Add(output.NewPubSubOutput(&mainWG, pubsubOutputOptions, textTemplateOptions, observability))
			outputs.Add(output.NewGitlabOutput(&mainWG, gitlabOutputOptions, textTemplateOptions, observability))

			inputs.Start(&mainWG, &outputs)
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

	flags.StringVar(&httpInputOptions.K8sURL, "http-in-k8s-url", httpInputOptions.K8sURL, "Http K8s url")
	flags.StringVar(&httpInputOptions.RancherURL, "http-in-rancher-url", httpInputOptions.RancherURL, "Http Rancher url")
	flags.StringVar(&httpInputOptions.AlertmanagerURL, "http-in-alertmanager-url", httpInputOptions.AlertmanagerURL, "Http Alertmanager url")
	flags.StringVar(&httpInputOptions.GitlabURL, "http-in-gitlab-url", httpInputOptions.GitlabURL, "Http Gitlab url")
	flags.StringVar(&httpInputOptions.DataDogURL, "http-in-datadog-url", httpInputOptions.DataDogURL, "Http DataDog url")
	flags.StringVar(&httpInputOptions.Site24x7URL, "http-in-site24x7-url", httpInputOptions.Site24x7URL, "Http Site24x7 url")
	flags.StringVar(&httpInputOptions.CloudflareURL, "http-in-cloudflare-url", httpInputOptions.CloudflareURL, "Http Cloudflare url")
	flags.StringVar(&httpInputOptions.GoogleURL, "http-in-google-url", httpInputOptions.GoogleURL, "Http Google url")
	flags.StringVar(&httpInputOptions.AWSURL, "http-in-aws-url", httpInputOptions.AWSURL, "Http AWS url")
	flags.StringVar(&httpInputOptions.CustomJsonURL, "http-in-customjson-url", httpInputOptions.CustomJsonURL, "Http CustomJson url")
	flags.StringVar(&httpInputOptions.Listen, "http-in-listen", httpInputOptions.Listen, "Http listen")
	flags.BoolVar(&httpInputOptions.Tls, "http-in-tls", httpInputOptions.Tls, "Http TLS")
	flags.StringVar(&httpInputOptions.Cert, "http-in-cert", httpInputOptions.Cert, "Http cert file or content")
	flags.StringVar(&httpInputOptions.Key, "http-in-key", httpInputOptions.Key, "Http key file or content")
	flags.StringVar(&httpInputOptions.Chain, "http-in-chain", httpInputOptions.Chain, "Http CA chain file or content")
	flags.StringVar(&httpInputOptions.HeaderTraceID, "http-in-header-trace-id", httpInputOptions.HeaderTraceID, "Http trace ID header")

	flags.StringVar(&pubsubInputOptions.Credentials, "pubsub-in-credentials", pubsubInputOptions.Credentials, "PubSub input credentials")
	flags.StringVar(&pubsubInputOptions.ProjectID, "pubsub-in-project-id", pubsubInputOptions.ProjectID, "PubSub input project ID")
	flags.StringVar(&pubsubInputOptions.Subscription, "pubsub-in-subscription", pubsubInputOptions.Subscription, "PubSub input subscription")

	flags.StringVar(&kafkaOutputOptions.Brokers, "kafka-out-brokers", kafkaOutputOptions.Brokers, "Kafka brokers")
	flags.StringVar(&kafkaOutputOptions.Topic, "kafka-out-topic", kafkaOutputOptions.Topic, "Kafka topic")
	flags.StringVar(&kafkaOutputOptions.ClientID, "kafka-out-client-id", kafkaOutputOptions.ClientID, "Kafka client id")
	flags.StringVar(&kafkaOutputOptions.Message, "kafka-out-message", kafkaOutputOptions.Message, "Kafka message template")
	flags.IntVar(&kafkaOutputOptions.FlushFrequency, "kafka-out-flush-frequency", kafkaOutputOptions.FlushFrequency, "Kafka Producer flush frequency")
	flags.IntVar(&kafkaOutputOptions.FlushMaxMessages, "kafka-out-flush-max-messages", kafkaOutputOptions.FlushMaxMessages, "Kafka Producer flush max messages")
	flags.IntVar(&kafkaOutputOptions.NetMaxOpenRequests, "kafka-out-net-max-open-requests", kafkaOutputOptions.NetMaxOpenRequests, "Kafka Net max open requests")
	flags.IntVar(&kafkaOutputOptions.NetDialTimeout, "kafka-out-net-dial-timeout", kafkaOutputOptions.NetDialTimeout, "Kafka Net dial timeout")
	flags.IntVar(&kafkaOutputOptions.NetReadTimeout, "kafka-out-net-read-timeout", kafkaOutputOptions.NetReadTimeout, "Kafka Net read timeout")
	flags.IntVar(&kafkaOutputOptions.NetWriteTimeout, "kafka-out-net-write-timeout", kafkaOutputOptions.NetWriteTimeout, "Kafka Net write timeout")

	flags.StringVar(&telegramOutputOptions.IDToken, "telegram-out-id-token", telegramOutputOptions.IDToken, "Telegram ID token")
	flags.StringVar(&telegramOutputOptions.ChatID, "telegram-out-chat-id", telegramOutputOptions.ChatID, "Telegram chat ID")
	flags.StringVar(&telegramOutputOptions.Message, "telegram-out-message", telegramOutputOptions.Message, "Telegram message template")
	flags.StringVar(&telegramOutputOptions.BotSelector, "telegram-out-bot-selector", telegramOutputOptions.BotSelector, "Telegram Bot selector template")
	flags.IntVar(&telegramOutputOptions.Timeout, "telegram-out-timeout", telegramOutputOptions.Timeout, "Telegram timeout")
	flags.StringVar(&telegramOutputOptions.ParseMode, "telegram-out-parse-mode", telegramOutputOptions.ParseMode, "Telegram parse mode")
	flags.StringVar(&telegramOutputOptions.AlertExpression, "telegram-out-alert-expression", telegramOutputOptions.AlertExpression, "Telegram alert expression")
	flags.BoolVar(&telegramOutputOptions.DisableNotification, "telegram-out-disable-notification", telegramOutputOptions.DisableNotification, "Telegram disable notification")
	flags.StringVar(&telegramOutputOptions.Forward, "telegram-out-forward", telegramOutputOptions.Forward, "Telegram forward regex pattern")
	flags.IntVar(&telegramOutputOptions.RateLimit, "telegram-rate-limit", telegramOutputOptions.RateLimit, "Ratelimit for telegram, per minute")

	flags.StringVar(&slackOutputOptions.Message, "slack-out-message", slackOutputOptions.Message, "Slack message template")
	flags.StringVar(&slackOutputOptions.ChannelSelector, "slack-out-channel-selector", slackOutputOptions.ChannelSelector, "Slack Channel selector template")
	flags.IntVar(&slackOutputOptions.Timeout, "slack-out-timeout", slackOutputOptions.Timeout, "Slack timeout")
	flags.StringVar(&slackOutputOptions.AlertExpression, "slack-out-alert-expression", slackOutputOptions.AlertExpression, "Slack alert expression")
	flags.StringVar(&slackOutputOptions.Token, "slack-out-token", slackOutputOptions.Token, "Slack token")
	flags.StringVar(&slackOutputOptions.Channel, "slack-out-channel", slackOutputOptions.Channel, "Slack channel")
	flags.StringVar(&slackOutputOptions.Forward, "slack-out-forward", slackOutputOptions.Forward, "Slack forward regex pattern")

	flags.StringVar(&workchatOutputOptions.URL, "workchat-out-url", workchatOutputOptions.URL, "Workchat URL")
	flags.StringVar(&workchatOutputOptions.Message, "workchat-out-message", workchatOutputOptions.Message, "Workchat message template")
	flags.StringVar(&workchatOutputOptions.URLSelector, "workchat-out-url-selector", workchatOutputOptions.URLSelector, "Workchat URL selector template")
	flags.IntVar(&workchatOutputOptions.Timeout, "workchat-out-timeout", workchatOutputOptions.Timeout, "Workchat timeout")
	flags.StringVar(&workchatOutputOptions.AlertExpression, "workchat-out-alert-expression", workchatOutputOptions.AlertExpression, "Workchat alert expression")
	flags.StringVar(&workchatOutputOptions.NotificationType, "workchat-out-notification-type", workchatOutputOptions.NotificationType, "Workchat notification type")

	flags.StringVar(&pubsubOutputOptions.Credentials, "pubsub-out-credentials", pubsubOutputOptions.Credentials, "PubSub output credentials")
	flags.StringVar(&pubsubOutputOptions.ProjectID, "pubsub-out-project-id", pubsubOutputOptions.ProjectID, "PubSub output project ID")
	flags.StringVar(&pubsubOutputOptions.TopicSelector, "pubsub-out-topic-selector", pubsubOutputOptions.TopicSelector, "PubSub output topic selector")
	flags.StringVar(&pubsubOutputOptions.Message, "pubsub-out-message", pubsubOutputOptions.Message, "PubSub output message")

	flags.StringVar(&gitlabOutputOptions.BaseURL, "gitlab-out-base-url", gitlabOutputOptions.BaseURL, "Gitlab output base URL")
	flags.StringVar(&gitlabOutputOptions.Token, "gitlab-out-token", gitlabOutputOptions.Token, "Gitlab output token")
	flags.StringVar(&gitlabOutputOptions.Projects, "gitlab-out-projects", gitlabOutputOptions.Projects, "Gitlab output projects")
	flags.StringVar(&gitlabOutputOptions.Variables, "gitlab-out-variables", gitlabOutputOptions.Variables, "Gitlab output variables")

	flags.StringVar(&grafanaRenderOptions.URL, "grafana-render-url", grafanaRenderOptions.URL, "Grafana render URL")
	flags.IntVar(&grafanaRenderOptions.Timeout, "grafana-render-timeout", grafanaRenderOptions.Timeout, "Grafan render timeout")
	flags.StringVar(&grafanaRenderOptions.Datasource, "grafana-render-datasource", grafanaRenderOptions.Datasource, "Grafana render datasource")
	flags.StringVar(&grafanaRenderOptions.ApiKey, "grafana-render-api-key", grafanaRenderOptions.ApiKey, "Grafana render API key")
	flags.StringVar(&grafanaRenderOptions.Org, "grafana-render-org", grafanaRenderOptions.Org, "Grafana render org")
	flags.IntVar(&grafanaRenderOptions.Period, "grafana-render-period", grafanaRenderOptions.Period, "Grafana render period in minutes")
	flags.IntVar(&grafanaRenderOptions.ImageWidth, "grafana-render-image-width", grafanaRenderOptions.ImageWidth, "Grafan render image width")
	flags.IntVar(&grafanaRenderOptions.ImageHeight, "grafana-render-image-height", grafanaRenderOptions.ImageHeight, "Grafan render image height")

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

	flags.StringVar(&datadogOptions.ApiKey, "datadog-api-key", datadogOptions.ApiKey, "DataDog API key")
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
	flags.StringVar(&datadogEventerOptions.Site, "datadog-eventer-site", datadogEventerOptions.Site, "DataDog eventer site")
	flags.StringVar(&datadogOutputOptions.Message, "datadog-out-message", datadogOutputOptions.Message, "DataDog message template")
	flags.StringVar(&datadogOutputOptions.AttributesSelector, "datadog-out-attributes-selector", datadogOutputOptions.AttributesSelector, "DataDog attributes selector template")

	flags.StringVar(&opentelemetryOptions.ServiceName, "opentelemetry-service-name", opentelemetryOptions.ServiceName, "Opentelemetry service name")
	flags.StringVar(&opentelemetryOptions.Environment, "opentelemetry-environment", opentelemetryOptions.Environment, "Opentelemetry environment")
	flags.StringVar(&opentelemetryOptions.Attributes, "opentelemetry-attributes", opentelemetryOptions.Attributes, "Opentelemetry attributes")
	flags.BoolVar(&opentelemetryOptions.Debug, "opentelemetry-debug", opentelemetryOptions.Debug, "Opentelemetry debug")
	flags.StringVar(&opentelemetryTracerOptions.AgentHost, "opentelemetry-tracer-agent-host", opentelemetryTracerOptions.AgentHost, "Opentelemetry tracer agent host")
	flags.IntVar(&opentelemetryTracerOptions.AgentPort, "opentelemetry-tracer-agent-port", opentelemetryTracerOptions.AgentPort, "Opentelemetry tracer agent port")
	flags.StringVar(&opentelemetryMeterOptions.AgentHost, "opentelemetry-meter-agent-host", opentelemetryMeterOptions.AgentHost, "Opentelemetry meter agent host")
	flags.IntVar(&opentelemetryMeterOptions.AgentPort, "opentelemetry-meter-agent-port", opentelemetryMeterOptions.AgentPort, "Opentelemetry meter agent port")
	flags.StringVar(&opentelemetryMeterOptions.Prefix, "opentelemetry-meter-prefix", opentelemetryMeterOptions.Prefix, "Opentelemetry meter prefix")

	flags.StringVar(&newrelicOptions.ApiKey, "newrelic-api-key", newrelicOptions.ApiKey, "NewRelic API key")
	flags.StringVar(&newrelicOptions.ServiceName, "newrelic-service-name", newrelicOptions.ServiceName, "NewRelic service name")
	flags.StringVar(&newrelicOptions.Environment, "newrelic-environment", newrelicOptions.Environment, "NewRelic environment")
	flags.StringVar(&newrelicOptions.Attributes, "newrelic-attributes", newrelicOptions.Attributes, "NewRelic attributes")
	flags.BoolVar(&newrelicOptions.Debug, "newrelic-debug", newrelicOptions.Debug, "NewRelic debug")
	flags.StringVar(&newrelicTracerOptions.Endpoint, "newrelic-tracer-endpoint", newrelicTracerOptions.Endpoint, "NewRelic tracer endpoint")
	flags.StringVar(&newrelicLoggerOptions.Endpoint, "newrelic-logger-endpoint", newrelicLoggerOptions.Endpoint, "NewRelic logger endpoint")
	flags.StringVar(&newrelicLoggerOptions.AgentHost, "newrelic-logger-agent-host", newrelicLoggerOptions.AgentHost, "NewRelic logger agent host")
	flags.IntVar(&newrelicLoggerOptions.AgentPort, "newrelic-logger-agent-port", newrelicLoggerOptions.AgentPort, "NewRelic logger agent port")
	flags.StringVar(&newrelicLoggerOptions.Level, "newrelic-logger-level", newrelicLoggerOptions.Level, "NewRelic logger level: info, warn, error, debug, panic")
	flags.StringVar(&newrelicMeterOptions.Endpoint, "newrelic-meter-endpoint", newrelicMeterOptions.Endpoint, "NewRelic meter endpoint")
	flags.StringVar(&newrelicMeterOptions.Prefix, "newrelic-meter-prefix", newrelicMeterOptions.Prefix, "NewRelic meter prefix")
	flags.StringVar(&newrelicEventerOptions.Endpoint, "newrelic-eventer-endpoint", newrelicEventerOptions.Endpoint, "NewRelic eventer endpoint")
	flags.StringVar(&newrelicOutputOptions.Message, "newrelic-out-message", newrelicOutputOptions.Message, "NewRelic message template")
	flags.StringVar(&newrelicOutputOptions.AttributesSelector, "newrelic-out-attributes-selector", newrelicOutputOptions.AttributesSelector, "NewRelic attributes selector template")

	flags.StringVar(&grafanaOptions.URL, "grafana-url", grafanaOptions.URL, "Grafana URL")
	flags.IntVar(&grafanaOptions.Timeout, "grafana-timeout", grafanaOptions.Timeout, "Grafan timeout")
	flags.StringVar(&grafanaOptions.ApiKey, "grafana-api-key", grafanaOptions.ApiKey, "Grafana API key")
	flags.StringVar(&grafanaOptions.Tags, "grafana-tags", grafanaOptions.Tags, "Grafana tags")
	flags.StringVar(&grafanaEventerOptions.Endpoint, "grafana-eventer-endpoint", grafanaEventerOptions.Endpoint, "Grafana eventer endpoint")
	flags.IntVar(&grafanaEventerOptions.Duration, "grafana-eventer-duration", grafanaEventerOptions.Duration, "Grafana eventer duration")
	flags.StringVar(&grafanaOutputOptions.Message, "grafana-out-message", grafanaOutputOptions.Message, "Grafana message template")
	flags.StringVar(&grafanaOutputOptions.AttributesSelector, "grafana-out-attributes-selector", grafanaOutputOptions.AttributesSelector, "Grafana attributes selector template")

	interceptSyscall()

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		logs.Error(err)
		os.Exit(1)
	}
}
