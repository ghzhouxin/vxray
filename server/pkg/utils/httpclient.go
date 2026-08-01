package utils

import (
	"net/http"
	"time"
)

const longHTTPTimeout = 5 * time.Minute

func LongRunningHTTPClient() *http.Client {
	return &http.Client{Timeout: longHTTPTimeout}
}
