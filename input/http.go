package input

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/processor"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
)

type HttpInputOptions struct {
	K8sURL          string
	RancherURL      string
	AlertmanagerURL string
	GitlabURL       string
	CustomJsonURL   string
	Listen          string
	Tls             bool
	Cert            string
	Key             string
	Chain           string
	HeaderTraceID   string
}

type HttpInput struct {
	options HttpInputOptions
	eventer sreCommon.Eventer
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	meter   sreCommon.Meter
	counter sreCommon.Counter
}

type HttpProcessHandleFunc = func(w http.ResponseWriter, r *http.Request)

func (h *HttpInput) startSpanFromRequest(r *http.Request) sreCommon.TracerSpan {

	traceID := r.Header.Get(h.options.HeaderTraceID)
	if utils.IsEmpty(traceID) {
		return h.tracer.StartSpan()
	}

	return h.tracer.StartSpanWithTraceID(traceID, "")
}

func (h *HttpInput) processURL(url string, mux *http.ServeMux, p common.HttpProcessor) {

	urls := strings.Split(url, ",")
	for _, url := range urls {

		mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

			span := h.startSpanFromRequest(r)
			span.SetCarrier(r.Header)
			span.SetTag("path", r.URL.Path)
			defer span.Finish()

			h.counter.Inc(r.URL.Path)
			p.HandleHttpRequest(w, r)
		})
	}
}

func (h *HttpInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {

	wg.Add(1)
	go func(wg *sync.WaitGroup) {

		defer wg.Done()
		h.logger.Info("Start http input...")

		var caPool *x509.CertPool
		var certificates []tls.Certificate

		if h.options.Tls {

			// load certififcate
			var cert []byte
			if _, err := os.Stat(h.options.Cert); err == nil {

				cert, err = ioutil.ReadFile(h.options.Cert)
				if err != nil {
					h.logger.Panic(err)
				}
			} else {
				cert = []byte(h.options.Cert)
			}

			// load key
			var key []byte
			if _, err := os.Stat(h.options.Key); err == nil {
				key, err = ioutil.ReadFile(h.options.Key)
				if err != nil {
					h.logger.Panic(err)
				}
			} else {
				key = []byte(h.options.Key)
			}

			// make pair from certificate and pair
			pair, err := tls.X509KeyPair(cert, key)
			if err != nil {
				h.logger.Panic(err)
			}

			certificates = append(certificates, pair)

			// load CA chain
			var chain []byte
			if _, err := os.Stat(h.options.Chain); err == nil {
				chain, err = ioutil.ReadFile(h.options.Chain)
				if err != nil {
					h.logger.Panic(err)
				}
			} else {
				chain = []byte(h.options.Chain)
			}

			// make pool of chains
			caPool = x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(chain) {
				h.logger.Debug("CA chain is invalid")
			}
		}

		mux := http.NewServeMux()
		processors := h.getProcessors(outputs)
		for u, p := range processors {
			h.processURL(u, mux, p)
		}

		listener, err := net.Listen("tcp", h.options.Listen)
		if err != nil {
			h.logger.Panic(err)
		}

		h.logger.Info("Http input is up. Listening...")

		srv := &http.Server{Handler: mux}

		if h.options.Tls {

			srv.TLSConfig = &tls.Config{
				Certificates: certificates,
				RootCAs:      caPool,
			}

			err = srv.ServeTLS(listener, "", "")
			if err != nil {
				h.logger.Panic(err)
			}
		} else {
			err = srv.Serve(listener)
			if err != nil {
				h.logger.Panic(err)
			}
		}

	}(wg)
}

func (h *HttpInput) getProcessors(outputs *common.Outputs) map[string]common.HttpProcessor {

	processors := make(map[string]common.HttpProcessor)

	if !utils.IsEmpty(h.options.K8sURL) {
		processors[h.options.K8sURL] = processor.NewK8sProcessor(outputs, h.logger, h.tracer, h.meter)
	}

	if !utils.IsEmpty(h.options.RancherURL) {
		processors[h.options.RancherURL] = processor.NewRancherProcessor(outputs, h.logger, h.tracer)
	}

	if !utils.IsEmpty(h.options.AlertmanagerURL) {
		processors[h.options.AlertmanagerURL] = processor.NewAlertmanagerProcessor(outputs, h.logger, h.tracer, h.meter)
	}

	if !utils.IsEmpty(h.options.GitlabURL) {
		processors[h.options.GitlabURL] = processor.NewGitlabProcessor(outputs, h.logger, h.tracer, h.meter)
	}

	if !utils.IsEmpty(h.options.CustomJsonURL) {
		processors[h.options.CustomJsonURL] = processor.NewCustomJsonProcessor(outputs, h.logger, h.tracer, h.meter)
	}

	return processors
}

func NewHttpInput(options HttpInputOptions, observability common.Observability) *HttpInput {

	meter := observability.Metrics()
	return &HttpInput{
		options: options,
		eventer: observability.Events(),
		tracer:  observability.Traces(),
		logger:  observability.Logs(),
		meter:   meter,
		counter: meter.Counter("requests", "Count of all http input requests", []string{"url"}, "http", "input"),
	}
}
