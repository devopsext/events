#!/bin/bash

#curl -sk -X POST -H "Content-type: application/json" -H "X-Gitlab-Event: Job Hook" -d @gitlab-job.json "http://localhost:8081/gitlab"
#curl -sk -X POST -H "Content-type: application/json" -H "X-Gitlab-Event: Pipeline Hook" -d @gitlab-pipeline.json "http://localhost:8081/gitlab"

#curl -sk -X POST -H "Content-type: application/json" -d @k8s.json "http://localhost:8081/k8s"

#curl -sk -X POST -H "Content-type: application/json" -d @alertmanager.json "http://localhost:8081/alertmanager"

curl -sk -X POST -H "Content-type: application/json" -d @datadog.json "http://localhost:8081/datadog"
