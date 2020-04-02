# Events

Http service which listen events in Kubernetes cluster, processes and send them to Kafka, Telegram or Collector.

It supports Go templates with extended syntax.

[![GoDoc](https://godoc.org/github.com/devopsext/events?status.svg)](https://godoc.org/github.com/devopsext/events)
[![build status](https://img.shields.io/travis/devopsext/events/master.svg?style=flat-square)](https://travis-ci.org/devopsext/events)

## Features

- 

## Build

```sh
git clone https://github.com/devopsext/events.git
cd events/
go build
```

## Example

```sh
./shipper --http-listen ":8081" --graphql-mode Playground \
          --clickhouse-host=HOST --clickhouse-password=PASSWORD --clickhouse-debug \
          --clickhouse-database-pattern "system" --clickhouse-table-pattern "(settings|tables)" \
          --log-template "{{.msg}}" --log-format stdout
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
  -h, --help                                help for events
      --http-cert string                    Http cert file or content
      --http-chain string                   Http CA chain file or content
      --http-k8s-url string                 Http K8s url
      --http-key string                     Http key file or content
      --http-listen string                  Http listen (default ":80")
      --http-rancher-url string             Http Rancher url
      --http-tls                            Http TLS
      --kafka-brokers string                Kafka brokers
      --kafka-client-id string              Kafka client id (default "events_kafka")
      --kafka-flush-frequency int           Kafka Producer flush frequency (default 1)
      --kafka-flush-max-messages int        Kafka Producer flush max messages (default 100)
      --kafka-message-template string       Kafka message template
      --kafka-net-dial-timeout int          Kafka Net dial timeout (default 30)
      --kafka-net-max-open-requests int     Kafka Net max open requests (default 5)
      --kafka-net-read-timeout int          Kafka Net read timeout (default 30)
      --kafka-net-write-timeout int         Kafka Net write timeout (default 30)
      --kafka-topic string                  Kafka topic (default "events")
      --log-format string                   Log format: json, text, stdout (default "text")
      --log-level string                    Log level: info, warn, error, debug, panic (default "info")
      --log-template string                 Log template (default "{{.func}} [{{.line}}]: {{.msg}}")
      --prometheus-listen string            Prometheus listen (default "127.0.0.1:8080")
      --prometheus-url string               Prometheus endpoint url (default "/metrics")
      --telegram-message-template string    Telegram message template
      --telegram-selector-template string   Telegram selector template
      --telegram-timeout int                Telegram timeout (default 30)
      --telegram-url string                 Telegram url
      --template-layout string              Template layout name
      --template-time-format string         Template time format (default "2006-01-02T15:04:05.999Z")
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
- EVENTS_HTTP_LISTEN
- EVENTS_HTTP_TLS
- EVENTS_HTTP_CERT
- EVENTS_HTTP_KEY
- EVENTS_HTTP_CHAIN

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

- EVENTS_COLLECTOR_ADDRESS
- EVENTS_COLLECTOR_MESSAGE_TEMPLATE
