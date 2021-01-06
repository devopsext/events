package common

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"
)

func IsEmpty(s string) bool {
	s1 := strings.TrimSpace(s)
	return len(s1) == 0
}

func MakeHttpClient(timeout int) *http.Client {

	var transport = &http.Transport{
		Dial:                (&net.Dialer{Timeout: time.Duration(timeout) * time.Second}).Dial,
		TLSHandshakeTimeout: time.Duration(timeout) * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	var client = &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}

	return client
}
