package common

type Event struct {
	Time     string      `json:"time"`
	TimeNano int64       `json:"timeNano"`
	Channel  string      `json:"channel"`
	Type     string      `json:"type"`
	Event    interface{} `json:"event"`
}
