package common

import "net/http"

type Processor interface {
	Type() string
}
type HttpProcessor interface {
	Processor
	HandleHttpRequest(w http.ResponseWriter, r *http.Request)
}
