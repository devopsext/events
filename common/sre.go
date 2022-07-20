package common

import (
	sre "github.com/devopsext/sre/common"
)

type Observability struct {
	logs    *sre.Logs
	traces  *sre.Traces
	metrics *sre.Metrics
	events  *sre.Events
}

func (o *Observability) Info(obj interface{}, args ...interface{}) {
	if o.logs != nil {
		o.logs.Info(obj, args)
	}
}

func (o *Observability) Warn(obj interface{}, args ...interface{}) {
	if o.logs != nil {
		o.logs.Warn(obj, args)
	}
}

func (o *Observability) Error(obj interface{}, args ...interface{}) {
	if o.logs != nil {
		o.logs.Error(obj, args)
	}
}

func (o *Observability) Logs() *sre.Logs {
	return o.logs
}

func (o *Observability) Traces() *sre.Traces {
	return o.traces
}

func (o *Observability) Metrics() *sre.Metrics {
	return o.metrics
}

func (o *Observability) Events() *sre.Events {
	return o.events
}

func NewObservability(logs *sre.Logs, traces *sre.Traces, metrics *sre.Metrics, events *sre.Events) *Observability {

	return &Observability{
		logs:    logs,
		traces:  traces,
		metrics: metrics,
		events:  events,
	}
}
