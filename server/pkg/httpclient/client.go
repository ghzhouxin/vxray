package httpclient

import (
	"net/http"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	LongTimeout    = 5 * time.Minute
)

func newClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

func Default() *http.Client {
	return newClient(DefaultTimeout)
}

func LongRunning() *http.Client {
	return newClient(LongTimeout)
}
