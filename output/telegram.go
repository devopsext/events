package output

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"

	"github.com/prometheus/client_golang/prometheus"
)

var telegramOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_telegram_output_count",
	Help: "Count of all telegram output count",
}, []string{"telegram_output_bot"})

type TelegramOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	template *render.TextTemplate
	selector *render.TextTemplate
	url      string
}

func (t *TelegramOutput) Send(o interface{}) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		if t.client == nil || t.template == nil {
			return
		}

		urls := t.url
		if t.selector != nil {

			b, err := t.selector.Execute(o)
			if err != nil {

				log.Error(err)
			} else {

				urls = b.String()
			}
		}

		if common.IsEmpty(urls) {

			log.Error(errors.New("Telegram urls are not found"))
			return
		}

		b, err := t.template.Execute(o)
		if err != nil {

			log.Error(err)
			return
		}

		message := b.String()

		if common.IsEmpty(message) {

			log.Debug("Message to Telegram is empty")
			return
		}

		arr := strings.Split(urls, "\n")

		for _, url := range arr {

			url = strings.TrimSpace(url)

			if !common.IsEmpty(url) {

				log.Debug("Message to Telegram (%s) => %s", url, message)

				reader := bytes.NewReader(b.Bytes())

				req, err := http.NewRequest("POST", url, reader)

				if err != nil {

					log.Error(err)
					return
				}

				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept-Encoding", "")

				resp, err := t.client.Do(req)

				if err != nil {
					log.Error(err)
				}

				defer resp.Body.Close()

				body, err := ioutil.ReadAll(resp.Body)

				if err != nil {
					log.Error(err)
				}

				// assume that url => https://api.telegram.org/bot508526210:sdsdfsdfsdf/sendMessage?chat_id=%s
				bot := ""

				arr := strings.Split(url, "/bot")
				if len(arr) > 1 {
					arr = strings.Split(arr[1], ":")
					if len(arr) > 0 {
						bot = arr[0]
					}
				}

				telegramOutputCount.WithLabelValues(bot).Inc()

				log.Debug(string(body))
			}
		}
	}()
}

func makeClient(url string, timeout int) *http.Client {

	if common.IsEmpty(url) {

		log.Debug("Telegram url is not defined. Skipped.")
		return nil
	}

	var transport = &http.Transport{
		Dial:                (&net.Dialer{Timeout: time.Duration(timeout) * time.Second}).Dial,
		TLSHandshakeTimeout: time.Duration(timeout) * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	var client = &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}

	return client
}

func NewTelegramOutput(wg *sync.WaitGroup, clientID string, template string, selector string, url string, timeout int) *TelegramOutput {

	return &TelegramOutput{
		wg:     wg,
		client: makeClient(url, timeout),
		//template: render.NewTextTemplate("telegram", template),
		//selector: render.NewTextTemplate("selector", selector),
		url: url,
	}
}

func init() {
	prometheus.Register(telegramOutputCount)
}
