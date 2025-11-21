package common

import (
	"encoding/json"
	"time"

	sreCommon "github.com/devopsext/sre/common"
)

type Event struct {
	Time    time.Time              `json:"time"`
	Channel string                 `json:"channel"`
	Type    string                 `json:"type"`
	Data    interface{}            `json:"data"`
	Via     map[string]interface{} `json:"via,omitempty"`
	logger  sreCommon.Logger
}

func (e *Event) JsonBytes() ([]byte, error) {

	bytes, err := json.Marshal(e)
	if err != nil {
		return []byte{}, err
	}
	return bytes, nil
}

func (e *Event) JsonObject() (interface{}, error) {

	bytes, err := e.JsonBytes()
	if err != nil {
		return nil, err
	}

	var object interface{}
	if err := json.Unmarshal(bytes, &object); err != nil {
		return "", err
	}
	return object, nil
}

func (e *Event) JsonMap() (map[string]interface{}, error) {

	bytes, err := e.JsonBytes()
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (e *Event) SetLogger(logger sreCommon.Logger) {
	e.logger = logger
}

func (e *Event) SetTime(time time.Time) {
	e.Time = time
}
