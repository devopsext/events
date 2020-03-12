package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	//"reflect"
	"sync"
	"syscall"

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

			/*var httpInput common.Input = input.NewHttpInput(httpInputOptions, processorOptions)

			if reflect.ValueOf(httpInput).IsNil() {
				log.Panic("Http input is invalid. Terminating...")
			}

			inputs := common.NewInputs()
			inputs.Add(&httpInput)

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