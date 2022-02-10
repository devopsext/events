package common

import "net/http"

type Processor interface {
	EventType() string
	HandleEvent(e *Event)
}

type HttpProcessor interface {
	Processor
	HandleHttpRequest(w http.ResponseWriter, r *http.Request)
}
