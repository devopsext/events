package output

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/client_golang/prometheus"
)

var slackOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_slack_output_count",
	Help: "Count of all slack outputs",
}, []string{"telegram_output_bot"})

type SlackOutputOptions struct {
	MessageTemplate  string
	SelectorTemplate string
	URL              string
	Timeout          int
	AlertExpression  string
}

type SlackOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.Grafana
	options  SlackOutputOptions
}

func (t *SlackOutput) Send(event *common.Event) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		if t.client == nil || t.message == nil {
			log.Error(errors.New("No client or message"))
			return
		}

		if event == nil {
			log.Error(errors.New("Event is empty"))
			return
		}

		if event.Data == nil {
			log.Error(errors.New("Event data is empty"))
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			log.Error(err)
			return
		}

		URLs := t.options.URL
		if t.selector != nil {

			b, err := t.selector.Execute(jsonObject)
			if err != nil {
				log.Error(err)
			} else {
				URLs = b.String()
			}
		}

		if common.IsEmpty(URLs) {
			log.Error(errors.New("Slack URLs are not found"))
			return
		}

		b, err := t.message.Execute(jsonObject)
		if err != nil {
			log.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			log.Debug("Slack message is empty")
			return
		}

		arr := strings.Split(URLs, "\n")

		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if common.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "K8sEvent":
				//t.sendMessage(URL, message)
			case "AlertmanagerEvent":

				if t.grafana != nil {

				} else {
					//t.sendMessage(URL, message)
				}
			}
		}
	}()
}

func NewSlackOutput(wg *sync.WaitGroup,
	options SlackOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaOptions render.GrafanaOptions) *SlackOutput {

	if common.IsEmpty(options.URL) {
		log.Debug("Slack URL is not defined. Skipped")
		return nil
	}

	return &SlackOutput{
		wg:       wg,
		client:   common.MakeHttpClient(options.Timeout),
		message:  render.NewTextTemplate("slack-message", options.MessageTemplate, templateOptions, options),
		selector: render.NewTextTemplate("slack-selector", options.SelectorTemplate, templateOptions, options),
		grafana:  render.NewGrafana(grafanaOptions),
		options:  options,
	}
}

func init() {
	prometheus.Register(slackOutputCount)
}
