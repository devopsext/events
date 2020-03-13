package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"

	"sync"
	"syscall"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/input"
	"github.com/devopsext/events/output"
	"github.com/devopsext/events/render"
	utils "github.com/devopsext/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var VERSION = "unknown"

var log = utils.GetLog()
var env = utils.GetEnvironment()

type rootOptions struct {
	LogFormat   string
	LogLevel    string
	LogTemplate string

	PrometheusURL    string
	PrometheusListen string
}

var rootOpts = rootOptions{

	LogFormat:   env.Get("EVENTS_LOG_FORMAT", "text").(string),
	LogLevel:    env.Get("EVENTS_LOG_LEVEL", "info").(string),
	LogTemplate: env.Get("EVENTS_LOG_TEMPLATE", "{{.func}} [{{.line}}]: {{.msg}}").(string),

	PrometheusURL:    env.Get("EVENTS_PROMETHEUS_URL", "/metrics").(string),
	PrometheusListen: env.Get("EVENTS_PROMETHEUS_LISTEN", "127.0.0.1:8080").(string),
}

var textTemplateOptions = render.TextTemplateOptions{

	TimeFormat: env.Get("EVENTS_TEMPLATE_TIME_FORMAT", "2006-01-02T15:04:05.999Z").(string),
	Layout:     env.Get("EVENTS_TEMPLATE_LAYOUT", "").(string),
}

var httpInputOptions = input.HttpInputOptions{

	K8sURL:     env.Get("EVENTS_HTTP_K8S_URL", "").(string),
	RancherURL: env.Get("EVENTS_HTTP_RANCHER_URL", "").(string),
	Listen:     env.Get("EVENTS_HTTP_LISTEN", ":80").(string),
	Tls:        env.Get("EVENTS_HTTP_TLS", false).(bool),
	Cert:       env.Get("EVENTS_HTTP_CERT", "").(string),
	Key:        env.Get("EVENTS_HTTP_KEY", "").(string),
	Chain:      env.Get("EVENTS_HTTP_CHAIN", "").(string),
}

var collectorOutputOptions = output.CollectorOutputOptions{

	Address:  env.Get("EVENTS_COLLECTOR_ADDRESS", "").(string),
	Template: env.Get("EVENTS_COLLECTOR_MESSAGE_TEMPLATE", "").(string),
}

func startMetrics(wg *sync.WaitGroup) {

	wg.Add(1)

	go func(wg *sync.WaitGroup) {

		defer wg.Done()

		log.Info("Start metrics...")

		http.Handle(rootOpts.PrometheusURL, promhttp.Handler())

		listener, err := net.Listen("tcp", rootOpts.PrometheusListen)
		if err != nil {
			log.Panic(err)
		}

		log.Info("Metrics are up. Listening...")

		err = http.Serve(listener, nil)
		if err != nil {
			log.Panic(err)
		}

	}(wg)
}

func interceptSyscall() {

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
	go func() {
		<-c
		log.Info("Exiting...")
		os.Exit(1)
	}()
}

func Execute() {

	rootCmd := &cobra.Command{
		Use:   "feeder",
		Short: "Feeder",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			log.CallInfo = true
			log.Init(rootOpts.LogFormat, rootOpts.LogLevel, rootOpts.LogTemplate)

		},
		Run: func(cmd *cobra.Command, args []string) {

			log.Info("Booting...")

			var wg sync.WaitGroup

			startMetrics(&wg)

			//render.SetTextTemplateConfig(vars, vars.TEMPLATE_TIME_FORMAT, vars.TEMPLATE_LAYOUT)

			var httpInput common.Input = input.NewHttpInput(httpInputOptions)
			if reflect.ValueOf(httpInput).IsNil() {
				log.Panic("Http input is invalid. Terminating...")
			}

			inputs := common.NewInputs()
			inputs.Add(&httpInput)

			var collectorOutput common.Output = output.NewCollectorOutput(&wg, collectorOutputOptions, textTemplateOptions)
			if reflect.ValueOf(collectorOutput).IsNil() {
				log.Warn("Collector input is invalid. Skipping...")
			}

			/*var kafkaOutput common.Output = output.NewKafkaOutput(&wg, vars.KAFKA_CLIENT_ID, vars.KAFKA_MESSAGE_TEMPLATE, vars.KAFKA_BROKERS, vars.KAFKA_TOPIC,
				vars.KAFKA_PRODUCER_FLUSH_FREQUENCY, vars.KAFKA_PRODUCER_FLUSH_MAX_MESSAGES,
				vars.KAFKA_NET_MAX_OPEN_REQUESTS, vars.KAFKA_NET_DIAL_TIMEOUT, vars.KAFKA_NET_READ_TIMEOUT, vars.KAFKA_NET_WRITE_TIMEOUT)
			var telegramOutput common.Output = output.NewTelegramOutput(&wg, vars.TELEGRAM_CLIENT_ID, vars.TELEGRAM_MESSAGE_TEMPLATE, vars.TELEGRAM_SELECTOR_TEMPLATE,
				vars.TELEGRAM_URL, vars.TELEGRAM_TIMEOUT)*/

			outputs := common.NewOutputs(textTemplateOptions.TimeFormat)
			outputs.Add(&collectorOutput)
			//outputs.Add(&kafkaOutput)
			//outputs.Add(&telegramOutput)

			/*

				var kafkaOutput common.Output = output.NewKafkaOutput(&wg, kafkaOutputOptions, kafkaOutputTopicsV1)

				if reflect.ValueOf(kafkaOutput).IsNil() {
					log.Panic("Kafka output is invalid. Terminating...")
				}

				outputs := common.NewOutputs()
				outputs.Add(&kafkaOutput)

				inputs.Start(&wg, outputs)
			*/
			wg.Wait()
		},
	}

	flags := rootCmd.PersistentFlags()

	flags.StringVar(&rootOpts.LogFormat, "log-format", rootOpts.LogFormat, "Log format: json, text, stdout")
	flags.StringVar(&rootOpts.LogLevel, "log-level", rootOpts.LogLevel, "Log level: info, warn, error, debug, panic")
	flags.StringVar(&rootOpts.LogTemplate, "log-template", rootOpts.LogTemplate, "Log template")

	flags.StringVar(&rootOpts.PrometheusURL, "prometheus-url", rootOpts.PrometheusURL, "Prometheus endpoint url")
	flags.StringVar(&rootOpts.PrometheusListen, "prometheus-listen", rootOpts.PrometheusListen, "Prometheus listen")

	flags.StringVar(&textTemplateOptions.TimeFormat, "template-time-format", textTemplateOptions.TimeFormat, "Template time format")
	flags.StringVar(&textTemplateOptions.Layout, "template-layout", textTemplateOptions.Layout, "Template layout name")

	flags.StringVar(&httpInputOptions.K8sURL, "http-k8s-url", httpInputOptions.K8sURL, "Http K8s url")
	flags.StringVar(&httpInputOptions.RancherURL, "http-rancher-url", httpInputOptions.RancherURL, "Http Rancher url")
	flags.StringVar(&httpInputOptions.Listen, "http-listen", httpInputOptions.Listen, "Http listen")
	flags.BoolVar(&httpInputOptions.Tls, "http-tls", httpInputOptions.Tls, "Http TLS")
	flags.StringVar(&httpInputOptions.Cert, "http-cert", httpInputOptions.Cert, "Http cert file or content")
	flags.StringVar(&httpInputOptions.Key, "http-key", httpInputOptions.Key, "Http key file or content")
	flags.StringVar(&httpInputOptions.Chain, "http-chain", httpInputOptions.Chain, "Http CA chain file or content")

	interceptSyscall()

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(VERSION)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
