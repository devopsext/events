package common

import (
	"encoding/json"
	"time"

	sreCommon "github.com/devopsext/sre/common"
)

type Event struct {
	Time        time.Time   `json:"time"`
	TimeNano    int64       `json:"timeNano"`
	Channel     string      `json:"channel"`
	Type        string      `json:"type"`
	Data        interface{} `json:"data"`
	spanContext sreCommon.TracerSpanContext
	logger      sreCommon.Logger
}

func (e *Event) JsonObject() (interface{}, error) {

	bytes, err := json.Marshal(e)
	if err != nil {
		if e.logger != nil {
			e.logger.Error(err)
		}
		return "", err
	}

	var object interface{}

	if err := json.Unmarshal(bytes, &object); err != nil {
		if e.logger != nil {
			e.logger.Error(err)
		}
		return "", err
	}

	return object, nil
}

func (e *Event) SetSpanContext(context sreCommon.TracerSpanContext) {
	e.spanContext = context
}

func (e *Event) GetSpanContext() sreCommon.TracerSpanContext {
	return e.spanContext
}

func (e *Event) SetLogger(logger sreCommon.Logger) {
	e.logger = logger
}

func (e *Event) SetTime(time time.Time) {
	e.Time = time
	e.TimeNano = time.UnixNano()
}
