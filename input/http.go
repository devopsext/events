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
	DataDogURL      string
	Site24x7URL     string
	CloudflareURL   string
	GoogleURL       string
	AWSURL          string
	CustomJsonURL   string
	Listen          string
	Tls             bool
	Cert            string
	Key             string
	Chain           string
	HeaderTraceID   string
}

type HttpInput struct {
	options    HttpInputOptions
	processors *common.Processors
	tracer     sreCommon.Tracer
	logger     sreCommon.Logger
	meter      sreCommon.Meter
	requests   sreCommon.Counter
	errors     sreCommon.Counter
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

			path := r.URL.Path

			span := h.startSpanFromRequest(r)
			span.SetCarrier(r.Header)
			span.SetTag("path", path)
			defer span.Finish()

			h.requests.Inc(path)
			err := p.HandleHttpRequest(w, r)
			if err != nil {
				h.errors.Inc(path)
			}
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
		processors := h.getProcessors(h.processors, outputs)
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

func (h *HttpInput) setProcessor(m map[string]common.HttpProcessor, url string, t string) {

	if !utils.IsEmpty(url) {
		p := h.processors.FindHttpProcessor(common.AsEventType(t))
		if p != nil {
			m[url] = p
		}
	}
}

func (h *HttpInput) getProcessors(processors *common.Processors, outputs *common.Outputs) map[string]common.HttpProcessor {

	m := make(map[string]common.HttpProcessor)
	h.setProcessor(m, h.options.K8sURL, processor.K8sProcessorType())
	h.setProcessor(m, h.options.AlertmanagerURL, processor.AlertmanagerProcessorType())
	h.setProcessor(m, h.options.GitlabURL, processor.GitlabProcessorType())
	h.setProcessor(m, h.options.RancherURL, processor.RancherProcessorType())
	h.setProcessor(m, h.options.DataDogURL, processor.DataDogProcessorType())
	h.setProcessor(m, h.options.Site24x7URL, processor.Site24x7ProcessorType())
	h.setProcessor(m, h.options.CloudflareURL, processor.CloudflareProcessorType())
	h.setProcessor(m, h.options.GoogleURL, processor.GoogleProcessorType())
	h.setProcessor(m, h.options.AWSURL, processor.AWSProcessorType())
	h.setProcessor(m, h.options.CustomJsonURL, processor.CustomJsonProcessorType())
	return m
}

func NewHttpInput(options HttpInputOptions, processors *common.Processors, observability *common.Observability) *HttpInput {

	meter := observability.Metrics()
	return &HttpInput{
		options:    options,
		processors: processors,
		tracer:     observability.Traces(),
		logger:     observability.Logs(),
		meter:      meter,
		requests:   meter.Counter("requests", "Count of all http input requests", []string{"url"}, "http", "input"),
		errors:     meter.Counter("errors", "Count of all http input errors", []string{"url"}, "http", "input"),
	}
}
