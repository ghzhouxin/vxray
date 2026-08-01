package utils

import (
	"net/http"
	"time"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	longHTTPTimeout    = 5 * time.Minute
)

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func DefaultHTTPClient() *http.Client {
	return newHTTPClient(defaultHTTPTimeout)
}

func LongRunningHTTPClient() *http.Client {
	return newHTTPClient(longHTTPTimeout)
}
