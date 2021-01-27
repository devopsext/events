# Events

Http service which inmplements an endpoint to listen events in Kubernetes cluster (webhook), as well as alerts from Alertmanager. By receiving events and alerts, the service processes them based on their kind and generates human readable message which sends to Kafka, Telegram, Slack or Workchat.

[![GoDoc](https://godoc.org/github.com/devopsext/events?status.svg)](https://godoc.org/github.com/devopsext/events)
[![build status](https://img.shields.io/travis/devopsext/events/master.svg?style=flat-square)](https://travis-ci.org/devopsext/events)

## Features

- Consume events from Kubernetes API, support kinds:
  - Namespace
  - Node
  - ReplicaSet
  - StatefulSet
  - DaemonSet
  - Secret
  - Ingress
  - CronJob
  - Job
  - ConfigMap
  - Role
  - Deployment
  - Service
  - Pod
- Consume alerts from Alertmanager
- Render alert images based on Grafana
- Support channels like: Kafka, Telegram, Slack and Workchat
- Support golang templates as patterns of messages for channels
- Support golang templates as patterns of channel selector
- Provide Prometheus metrics out of the box


## Build

```sh
git clone https://github.com/devopsext/events.git
cd events/
go build
```

## Example

```sh

```

## Usage

```
Events command

Usage:
  events [flags]
  events [command]

Available Commands:
  help        Help about any command
  version     Print the version number

Flags:
      --grafana-api-key string                 Grafana API key
      --grafana-datasource string              Grafana datasource (default "Prometheus")
      --grafana-image-height int               Grafan image height (default 640)
      --grafana-image-width int                Grafan image width (default 1280)
      --grafana-org string                     Grafana org (default "1")
      --grafana-period int                     Grafana period in minutes (default 60)
      --grafana-timeout int                    Grafan timeout (default 60)
      --grafana-url string                     Grafana URL
  -h, --help                                   help for events
      --http-alertmanager-url string           Http Alertmanager url
      --http-cert string                       Http cert file or content
      --http-chain string                      Http CA chain file or content
      --http-k8s-url string                    Http K8s url
      --http-key string                        Http key file or content
      --http-listen string                     Http listen (default ":80")
      --http-rancher-url string                Http Rancher url
      --http-tls                               Http TLS
      --kafka-brokers string                   Kafka brokers
      --kafka-client-id string                 Kafka client id (default "events_kafka")
      --kafka-flush-frequency int              Kafka Producer flush frequency (default 1)
      --kafka-flush-max-messages int           Kafka Producer flush max messages (default 100)
      --kafka-message-template string          Kafka message template
      --kafka-net-dial-timeout int             Kafka Net dial timeout (default 30)
      --kafka-net-max-open-requests int        Kafka Net max open requests (default 5)
      --kafka-net-read-timeout int             Kafka Net read timeout (default 30)
      --kafka-net-write-timeout int            Kafka Net write timeout (default 30)
      --kafka-topic string                     Kafka topic (default "events")
      --log-format string                      Log format: json, text, stdout (default "stdout")
      --log-level string                       Log level: info, warn, error, debug, panic (default "debug")
      --log-template string                    Log template (default "{{.msg}}")
      --prometheus-listen string               Prometheus listen (default "127.0.0.1:8080")
      --prometheus-url string                  Prometheus endpoint url (default "/metrics")
      --slack-alert-expression string          Slack alert expression (default "g0.expr")
      --slack-message-template string          Slack message template
      --slack-selector-template string         Slack selector template
      --slack-timeout int                      Slack timeout (default 30)
      --slack-url string                       Slack URL
      --telegram-alert-expression string       Telegram alert expression (default "g0.expr")
      --telegram-disable-notification string   Telegram disable notification (default "false")
      --telegram-message-template string       Telegram message template
      --telegram-selector-template string      Telegram selector template
      --telegram-timeout int                   Telegram timeout (default 30)
      --telegram-url string                    Telegram URL
      --template-time-format string            Template time format (default "2006-01-02T15:04:05.999Z")
      --workchat-alert-expression string       Workchat alert expression (default "g0.expr")
      --workchat-message-template string       Workchat message template
      --workchat-notification-type string      Workchat notification type (default "REGULAR")
      --workchat-selector-template string      Workchat selector template
      --workchat-timeout int                   Workchat timeout (default 30)
      --workchat-url string                    Workchat URL

```

## Environment variables

For containerization purpose all command switches have environment variables analogs.

- EVENTS_LOG_FORMAT
- EVENTS_LOG_LEVEL
- EVENTS_LOG_TEMPLATE

- EVENTS_PROMETHEUS_URL
- EVENTS_PROMETHEUS_LISTEN

- EVENTS_TEMPLATE_TIME_FORMAT
- EVENTS_HTTP_K8S_URL
- EVENTS_HTTP_RANCHER_URL
- EVENTS_HTTP_ALERTMANAGER_URL
- EVENTS_HTTP_LISTEN
- EVENTS_HTTP_TLS
- EVENTS_HTTP_CERT
- EVENTS_HTTP_KEY
- EVENTS_HTTP_CHAIN

- EVENTS_COLLECTOR_ADDRESS
- EVENTS_COLLECTOR_MESSAGE_TEMPLATE

- EVENTS_KAFKA_CLIEND_ID
- EVENTS_KAFKA_MESSAGE_TEMPLATE
- EVENTS_KAFKA_BROKERS
- EVENTS_KAFKA_TOPIC
- EVENTS_KAFKA_FLUSH_FREQUENCY
- EVENTS_KAFKA_FLUSH_MAX_MESSAGES
- EVENTS_KAFKA_NET_MAX_OPEN_REQUESTS
- EVENTS_KAFKA_NET_DIAL_TIMEOUT
- EVENTS_KAFKA_NET_READ_TIMEOUT
- EVENTS_KAFKA_NET_WRITE_TIMEOUT

- EVENTS_TELEGRAM_MESSAGE_TEMPLATE
- EVENTS_TELEGRAM_SELECTOR_TEMPLATE
- EVENTS_TELEGRAM_URL
- EVENTS_TELEGRAM_TIMEOUT
- EVENTS_TELEGRAM_ALERT_EXPRESSION
- EVENTS_TELEGRAM_DISABLE_NOTIFICATION

- EVENTS_SLACK_MESSAGE_TEMPLATE
- EVENTS_SLACK_SELECTOR_TEMPLATE
- EVENTS_SLACK_URL
- EVENTS_SLACK_TIMEOUT
- EVENTS_SLACK_ALERT_EXPRESSION

- EVENTS_WORKCHAT_MESSAGE_TEMPLATE
- EVENTS_WORKCHAT_SELECTOR_TEMPLATE
- EVENTS_WORKCHAT_URL
- EVENTS_WORKCHAT_TIMEOUT
- EVENTS_WORKCHAT_ALERT_EXPRESSION
- EVENTS_WORKCHAT_NOTIFICATION_TYPE

- EVENTS_GRAFANA_URL
- EVENTS_GRAFANA_TIMEOUT
- EVENTS_GRAFANA_DATASOURCE
- EVENTS_GRAFANA_API_KEY
- EVENTS_GRAFANA_ORG
- EVENTS_GRAFANA_PERIOD
- EVENTS_GRAFANA_IMAGE_WIDTH
- EVENTS_GRAFANA_IMAGE_HEIGHT
