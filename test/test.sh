#!/bin/bash

#curl -sk -X POST -H "Content-type: application/json" -H "X-Gitlab-Event: Job Hook" -d @gitlab-job.json "http://localhost:80/gitlab"
#curl -sk -X POST -H "Content-type: application/json" -H "X-Gitlab-Event: Pipeline Hook" -d @gitlab-pipeline.json "http://localhost:80/gitlab"

#curl -sk -X POST -H "Content-type: application/json" -d @k8s.json "http://localhost:80/k8s"

#curl -sk -X POST -H "Content-type: application/json" -d @alertmanager.json "http://localhost:80/alertmanager"
curl -sk -X POST -H "Content-type: application/json" -d @zabbix.json "http://localhost:80/zabbix"

#curl -sk -X POST -H "Content-type: application/json" -d @datadog-triggered.json "http://localhost:80/datadog"
#curl -sk -X POST -H "Content-type: application/json" -d @datadog-recovered.json "http://localhost:80/datadog"

#curl -sk -X POST -H "Content-type: application/json" -d @site24x7.json "http://localhost:80/site24x7"

#curl -sk -X POST -H "Content-type: application/json" -d @cloudflare.json "http://localhost:80/cloudflare"

#curl -sk -X POST -H "Content-type: application/json" -d @google.json "http://localhost:80/google"

#curl -sk -X POST -H "Content-type: application/json" -d @aws.json "http://localhost:80/aws.amazon.com"
