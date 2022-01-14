package common

import "net/http"

type HttpProcessor interface {
	HandleHttpRequest(w http.ResponseWriter, r *http.Request)
}
