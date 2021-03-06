package s3persist

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/smartystreets/projector/persist"
)

type GetRetryClient struct {
	inner   persist.HTTPClient
	retries int
	sleeper func(time.Duration)
}

// FUTURE: We may want to consider a ShutdownClient that sits just under
// the RetryClient. This makes it possible for a shutdown signal to break
// a retry loop because the Shutdown client would retry success (HTTP 200)
// or perhaps HTTP 404?

func NewGetRetryClient(inner persist.HTTPClient, retries int, sleeper func(time.Duration)) *GetRetryClient {
	return &GetRetryClient{inner: inner, retries: retries, sleeper: sleeper}
}

func (this *GetRetryClient) Do(request *http.Request) (*http.Response, error) {
	if request.Method != "GET" {
		return this.inner.Do(request)
	}

	for current := 0; current <= this.retries; current++ {
		response, err := this.inner.Do(request)
		if err == nil && response.StatusCode == http.StatusOK {
			return response, nil
		} else if err == nil && response.StatusCode == http.StatusNotFound {
			return response, nil
		} else if err != nil {
			log.Println("[WARN] Unexpected response from target storage:", err)
		} else if response.Body != nil {
			log.Printf("[WARN] Target host rejected request ('%s'):\n%s\n", request.URL.Path, readResponse(response))
		}
		this.sleeper(time.Second * 5)
	}
	return nil, errors.New("Max retries exceeded. Unable to connect.")
}
