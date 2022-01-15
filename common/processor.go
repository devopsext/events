package common

import "net/http"

type HttpProcessor interface {
	Type() string
	HandleHttpRequest(w http.ResponseWriter, r *http.Request)
}
