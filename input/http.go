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
	"github.com/devopsext/utils"
	"github.com/prometheus/client_golang/prometheus"
)

var httpInputRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_http_input_requests",
	Help: "Count of all http input requests",
}, []string{"input_url"})

type HttpInputOptions struct {
	K8sURL     string
	RancherURL string
	Listen     string
	Tls        bool
	Cert       string
	Key        string
	Chain      string
}

type HttpInput struct {
	options HttpInputOptions
}

func (h *HttpInput) Start(wg *sync.WaitGroup, outputs *common.Outputs) {

	wg.Add(1)

	go func(wg *sync.WaitGroup) {

		defer wg.Done()

		log.Info("Start http input...")

		var caPool *x509.CertPool
		var certificates []tls.Certificate

		if h.options.Tls {

			// load certififcate
			var cert []byte
			if _, err := os.Stat(h.options.Cert); err == nil {

				cert, err = ioutil.ReadFile(h.options.Cert)
				if err != nil {
					log.Panic(err)
				}
			} else {
				cert = []byte(h.options.Cert)
			}

			// load key
			var key []byte
			if _, err := os.Stat(h.options.Key); err == nil {

				key, err = ioutil.ReadFile(h.options.Key)
				if err != nil {
					log.Panic(err)
				}
			} else {
				key = []byte(h.options.Key)
			}

			// make pair from certificate and pair
			pair, err := tls.X509KeyPair(cert, key)
			if err != nil {
				log.Panic(err)
			}

			certificates = append(certificates, pair)

			// load CA chain
			var chain []byte
			if _, err := os.Stat(h.options.Chain); err == nil {

				chain, err = ioutil.ReadFile(h.options.Chain)
				if err != nil {
					log.Panic(err)
				}
			} else {
				chain = []byte(h.options.Chain)
			}

			// make pool of chains
			caPool = x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(chain) {
				log.Debug("CA chain is invalid")
			}
		}

		mux := http.NewServeMux()

		if !utils.IsEmpty(h.options.K8sURL) {

			urls := strings.Split(h.options.K8sURL, ",")
			for _, url := range urls {

				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

					httpInputRequests.WithLabelValues(r.URL.Path).Inc()
					processor.NewK8sProcessor(outputs).HandleHttpRequest(w, r)
				})
			}
		}

		if !utils.IsEmpty(h.options.RancherURL) {

			urls := strings.Split(h.options.RancherURL, ",")
			for _, url := range urls {

				mux.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {

					httpInputRequests.WithLabelValues(url).Inc()
					processor.NewK8sProcessor(outputs).HandleHttpRequest(w, r)
				})
			}
		}

		listener, err := net.Listen("tcp", h.options.Listen)
		if err != nil {
			log.Panic(err)
		}

		log.Info("Http input is up. Listening...")

		srv := &http.Server{Handler: mux}

		if h.options.Tls {

			srv.TLSConfig = &tls.Config{
				Certificates: certificates,
				RootCAs:      caPool,
			}

			err = srv.ServeTLS(listener, "", "")
			if err != nil {
				log.Panic(err)
			}
		} else {
			err = srv.Serve(listener)
			if err != nil {
				log.Panic(err)
			}
		}

	}(wg)
}

func NewHttpInput(options HttpInputOptions) *HttpInput {

	return &HttpInput{
		options: options,
	}
}

func init() {
	prometheus.Register(httpInputRequests)
}