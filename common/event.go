package common

import (
	"encoding/json"
)

type Event struct {
	Time     string      `json:"time"`
	TimeNano int64       `json:"timeNano"`
	Channel  string      `json:"channel"`
	Type     string      `json:"type"`
	Data     interface{} `json:"data"`
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
