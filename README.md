# Events

The service which implements an endpoint to listen events from Kubernetes cluster, alerts from Alertmanager, events from DataDog, Site24x7, Cloudflare or Google alerts. By receiving events and alerts, the service processes them based on their kind and generates human readable message which sends to Kafka, Telegram, Slack, Workchat, Grafana, DataDog, NewRelic, PubSub.

[![GoDoc](https://godoc.org/github.com/devopsext/events?status.svg)](https://godoc.org/github.com/devopsext/events)
[![go report](	https://goreportcard.com/badge/github.com/devopsext/events)](https://goreportcard.com/report/github.com/devopsext/events)
[![codecov](https://codecov.io/gh/devopsext/events/branch/master/graph/badge.svg?token=EDXEG24VZV)](https://codecov.io/gh/devopsext/events)
[![build status](https://travis-ci.com/devopsext/events.svg?branch=main)](https://app.travis-ci.com/github/devopsext/events)

## Features

- Consume events from Kubernetes API, support kinds: Namespace, Node, ReplicaSet, StatefulSet, DaemonSet, Secret, Ingress, CronJob, Job, ConfigMap, Role, Deployment, Service, Pod
- Consume alerts from Alertmanager and render alert images based on Grafana
- Support golang templates as patterns of messages for channels and channel selectors
- Template functions: regexReplaceAll, regexMatch, replaceAll, toLower, toTitle, toUpper, toJSON, split, join, isEmpty, getEnv, getVar, timeFormat, jsonEscape, toString
- Support channels like: Kafka, Telegram, Slack, Workchat. All templates in place
- Provide SRE metrics, logs, traces out of the box (see [devopsext/sre](https://github.com/devopsext/sre))

## Build

Set proper GOROOT and PATH variables
```sh
export GOROOT="$HOME/go/root/1.17.4"
export PATH="$PATH:$GOROOT/bin"
```

Clone repository
```sh
git clone https://github.com/devopsext/events.git
cd events/
go build
```

## Example

<details>
  <summary>Create Kubernetes API event json file</summary>

```sh
cat <<EOF > k8s.json
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1beta1",
  "request": {
    "uid": "23172a7a-f4c6-11e9-953e-0050568aa55b",
    "kind": {
      "group": "",
      "version": "v1",
      "kind": "Pod"
    },
    "resource": {
      "group": "",
      "version": "v1",
      "resource": "pods"
    },
    "namespace": "nodegroup",
    "operation": "CREATE",
    "userInfo": {
      "username": "some-user",
      "uid": "380bb127-e96f-11e8-ae7d-0050568a9a8e",
      "groups": [
        "system:serviceaccounts",
        "system:serviceaccounts:kube-system",
        "system:authenticated"
      ]
    },
    "object": {
      "metadata": {
        "name": "someservice-php-order-1571746740-glbhp",
        "generateName": "someservice-php-order-1571746740-",
        "namespace": "nodegroup",
        "uid": "23171eb4-f4c6-11e9-953e-0050568aa55b",
        "creationTimestamp": "2019-10-22T12:19:04Z",
        "labels": {
          "controller-uid": "231132ee-f4c6-11e9-953e-0050568aa55b",
          "job-name": "someservice-php-order-1571746740",
          "k8s-app": "someservice-php-order",
          "platform.collector/injected": "true",
          "version": "v0.4"
        },
        "annotations": {
          "app": "someservice-php-order",
          "prometheus.io/path": "/metrics",
          "prometheus.io/port": "60000",
          "prometheus.io/scrape": "true"
        },
        "ownerReferences": [
          {
            "apiVersion": "batch/v1",
            "kind": "Job",
            "name": "someservice-php-order-1571746740",
            "uid": "231132ee-f4c6-11e9-953e-0050568aa55b",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ]
      },
      "spec": {
        "volumes": [
          {
            "name": "someservice-php-env-file-volume",
            "configMap": {
              "name": "someservice-php-env-file",
              "defaultMode": 420
            }
          },
          {
            "name": "default-token-mn7zd",
            "secret": {
              "secretName": "default-token-mn7zd",
              "defaultMode": 420
            }
          },
          {
            "name": "dockersock",
            "hostPath": {
              "path": "/var/run/docker.sock",
              "type": ""
            }
          },
          {
            "name": "platform-collector-token",
            "secret": {
              "secretName": "platform-collector-token",
              "defaultMode": 420
            }
          }
        ],
        "containers": [
          {
            "name": "someservice-php-order",
            "image": "someregistry.com/someservice-php:v0.4",
            "command": [
              "/bin/bash",
              "-c",
              "cd /var/www ; php -d memory_limit=512M artisan transform:order; echo \"Done\"; sleep 3"
            ],
            "env": [
              {
                "name": "POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "metadata.name"
                  }
                }
              }
            ],
            "resources": {
              "limits": {
                "cpu": "1",
                "memory": "200Mi"
              },
              "requests": {
                "cpu": "500m",
                "memory": "150Mi"
              }
            },
            "volumeMounts": [
              {
                "name": "someservice-php-env-file-volume",
                "mountPath": "/env"
              },
              {
                "name": "default-token-mn7zd",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "IfNotPresent"
          },
          {
            "name": "collector",
            "image": "collector/pod:1.9.3.11-1.1.0",
            "env": [
              {
                "name": "KAFKA_BROKERS",
                "value": "broker:9092"
              },
              {
                "name": "POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "metadata.name"
                  }
                }
              },
              {
                "name": "COLLECTOR_GLOBAL_TAGS_ORCHESTRATION",
                "value": "k8s.test.env"
              }
            ],
            "resources": {},
            "volumeMounts": [
              {
                "name": "platform-collector-token",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              },
              {
                "name": "dockersock",
                "readOnly": true,
                "mountPath": "/var/run/docker.sock"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "restartPolicy": "Never",
        "terminationGracePeriodSeconds": 30,
        "dnsPolicy": "ClusterFirst",
        "nodeSelector": {
          "platform.isolation/nodegroup": "nodegroup"
        },
        "serviceAccountName": "default",
        "serviceAccount": "default",
        "securityContext": {},
        "imagePullSecrets": [
          {
            "name": "registry.exness.io"
          }
        ],
        "schedulerName": "default-scheduler",
        "tolerations": [
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          }
        ],
        "priority": 0
      },
      "status": {
        "phase": "Pending",
        "qosClass": "Burstable"
      }
    },
    "oldObject": null,
    "dryRun": false
  }
}
EOF
```
</details>

<details>
  <summary>Create Alertmanager alert json file</summary>

```sh
cat <<EOF > alertmanager.json
{
  "receiver": "events",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "Process Open FDS 2",
        "app": "prometheus",
        "instance": "10.42.0.5:9090",
        "kubernetes_namespace": "default",
        "kubernetes_pod_name": "prometheus-0",
        "severity": "some",
        "unit": "short",
        "minutes": "10",
        "statefulset_kubernetes_io_pod_name": "prometheus-0"
      },
      "annotations": {
        "summary": "High process Open FDS"
      },
      "startsAt": "2020-12-22T16:42:47.056441315Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus-0:9090/graph?g0.expr=rate(process_cpu_seconds_total[1m]) > 0.004&g0.tab=1",
      "fingerprint": "f8767e67485c740c"
    }
  ],
  "groupLabels": {
    "alertname": "Process Open FDS"
  },
  "commonLabels": {
    "alertname": "Process Open FDS",
    "app": "prometheus",
    "instance": "10.42.0.5:9090",
    "kubernetes_namespace": "default",
    "kubernetes_pod_name": "prometheus-0",
    "severity": "some",
    "statefulset_kubernetes_io_pod_name": "prometheus-0"
  },
  "commonAnnotations": {
    "summary": "High process Open FDS"
  },
  "externalURL": "http://alertmanager-db66d4578-dm696:9093",
  "version": "4",
  "groupKey": "{}/{}:{alertname=\"Process Open FDS\"}"
}
EOF
```
</details>

<details>
  <summary>Provide environment common variable</summary>
<br>
In a case of Grafana images based on alert rules which come from Alertmanager, setup environment variables for convinience. You can use command switches for that, but for the sake of simplicity, environment variables should be provided. 

```sh
export EVENTS_GRAFANA_URL="Place Grafana URL in case of Alertmanger images or leave it empty"
export EVENTS_GRAFANA_API_KEY="Place Grafana API key"
export EVENTS_STDOUT_FORMAT="template"
export EVENTS_STDOUT_LEVEL="debug"
export EVENTS_STDOUT_TEMPLATE="{{.msg}}"
```
</details>

<details>
  <summary>Run Events with Telegram channel</summary>

```sh
export TELEGRAM_BOT="Place Telegram bot"
export TELEGRAM_CHAT_ID="Place Telegram chat ID"
```

```sh




./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
  --telegram-out-id-token=${TELEGRAM_BOT} --telegram-out-chat-id=${TELEGRAM_CHAT_ID} \
  --telegram-message-template "{{- define \"telegram-message\"}}{{ toJSON . }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
  --telegram-out-id-token=${TELEGRAM_BOT} --telegram-out-chat-id=${TELEGRAM_CHAT_ID} --telegram-out-message "telegram.message"
```

</details>

<details>
  <summary>Run Events with Slack channel</summary>

```sh
export SLACK_TOKEN="Place Slack token"
export SLACK_CHANNELS="Place Slack channels"
```

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "{{- define \"slack-message\"}}{{ toJSON . }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "slack.message"
```
</details>

<details>
  <summary>Run Events with Workchat channel</summary>

```sh
export WORKCHAT_TOKEN="Place Workchat access token"
export WORKCHAT_RECIPIENT="Place Wotkchat thread group"
```

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "{{- define \"workchat-message\"}}{{ replaceAll \"\\\"\" \"\" (toJSON .) }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "workchat.message"
```

</details>

<details>
  <summary>Run Events with Telegram, Slack and Workchat simultaniously</summary>

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --telegram-url "https://api.telegram.org/bot${TELEGRAM_BOT}/sendMessage?chat_id=${TELEGRAM_CHAT_ID}" \
         --telegram-message-template "telegram.message" \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "slack.message" \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "workchat.message"
```

</details>

<details>
  <summary>Test Alertmanager endpoint</summary>

```sh
curl -X POST -H 'Content-type: application/json' -d @alertmanager.json http://127.0.0.1:8081/alertmanager
```

```json
{"Message":"OK"}
```
</details>

<details>
  <summary>Test Kubernetes API endpoint</summary>

```sh
curl -X POST -H 'Content-type: application/json' -d @k8s.json http://127.0.0.1:8081/k8s
```

```json
{"response":{"uid":"23172a7a-f4c6-11e9-953e-0050568aa55b","allowed":true}}
```
</details>
