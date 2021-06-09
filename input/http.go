package input

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
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
	Listen          string
	Tls             bool
	Cert            string
	Key             string
	Chain           string
	HeaderTraceID   string
}

type HttpInput struct {
	options  HttpInputOptions
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	metricer sreCommon.Metricer
	counter  sreCommon.Counter
}

func (h *HttpInput) startSpanFromRequest(r *http.Request) sreCommon.TracerSpan {

	s := r.Header.Get(h.options.HeaderTraceID)
	if utils.IsEmpty(s) {
		return h.tracer.StartSpan()
	}

	traceID, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		h.logger.Error("Invalid %s with value %s: %s", h.options.HeaderTraceID, s, err)
		return h.tracer.StartSpan()
	}

	return h.tracer.StartSpanWithTraceID(uint64(traceID))
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

		if !utils.IsEmpty(h.options.K8sURL) {

			urls := strings.Split(h.options.K8sURL, ",")
			for _, url := range urls {

				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

					span := h.startSpanFromRequest(r)
					span.SetCarrier(r.Header)
					span.SetTag("path", r.URL.Path)
					defer span.Finish()

					h.counter.Inc(r.URL.Path)
					processor.NewK8sProcessor(outputs, h.logger, h.tracer, h.metricer).HandleHttpRequest(w, r)
				})
			}
		}

		if !utils.IsEmpty(h.options.RancherURL) {

			urls := strings.Split(h.options.RancherURL, ",")
			for _, url := range urls {

				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

					span := h.startSpanFromRequest(r)
					span.SetCarrier(r.Header)
					span.SetTag("path", r.URL.Path)
					defer span.Finish()

					h.counter.Inc(r.URL.Path)
					processor.NewRancherProcessor(outputs, h.logger, h.tracer).HandleHttpRequest(w, r)
				})
			}
		}

		if !utils.IsEmpty(h.options.AlertmanagerURL) {

			urls := strings.Split(h.options.AlertmanagerURL, ",")
			for _, url := range urls {

				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

					span := h.startSpanFromRequest(r)
					span.SetCarrier(r.Header)
					span.SetTag("path", r.URL.Path)
					defer span.Finish()

					h.counter.Inc(r.URL.Path)
					processor.NewAlertmanagerProcessor(outputs, h.logger, h.tracer, h.metricer).HandleHttpRequest(w, r)
				})
			}
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

func NewHttpInput(options HttpInputOptions, logger sreCommon.Logger, tracer sreCommon.Tracer, metricer sreCommon.Metricer) *HttpInput {

	return &HttpInput{
		options:  options,
		tracer:   tracer,
		logger:   logger,
		metricer: metricer,
		counter:  metricer.Counter("requests", "Count of all http input requests", []string{"url"}, "http", "input"),
	}
}
