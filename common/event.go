package common

import (
	"encoding/json"

	"github.com/opentracing/opentracing-go"
)

type Event struct {
	Time       string      `json:"time"`
	TimeNano   int64       `json:"timeNano"`
	Channel    string      `json:"channel"`
	Type       string      `json:"type"`
	Data       interface{} `json:"data"`
	spanConext opentracing.SpanContext
}

func (e *Event) JsonObject() (interface{}, error) {

	bytes, err := json.Marshal(e)
	if err != nil {
		log.Error(err)
		return "", err
	}

	var object interface{}

	if err := json.Unmarshal(bytes, &object); err != nil {
		log.Error(err)
		return "", err
	}

	return object, nil
}

func (e *Event) SetSpanContext(spanConext opentracing.SpanContext) {
	e.spanConext = spanConext
}

func (e *Event) GetSpanContext() opentracing.SpanContext {
	return e.spanConext
}
