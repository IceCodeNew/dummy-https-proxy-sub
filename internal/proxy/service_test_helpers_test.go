package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// blockingHTTPClient delays responses so tests can coordinate concurrent requests.
type blockingHTTPClient struct {
	body string

	once    sync.Once
	start   chan struct{}
	unblock chan struct{}

	mu    sync.Mutex
	calls int
}

// newBlockingHTTPClient initializes a gateable HTTP client backed by the provided body.
func newBlockingHTTPClient(body string) *blockingHTTPClient {
	return &blockingHTTPClient{
		body:    body,
		start:   make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (c *blockingHTTPClient) Do(*http.Request) (*http.Response, error) {
	c.once.Do(func() { close(c.start) })

	c.mu.Lock()
	c.calls++
	c.mu.Unlock()

	<-c.unblock

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Header:     make(http.Header),
	}, nil
}

// waitUntilStarted blocks until the first request reaches Do.
func (c *blockingHTTPClient) waitUntilStarted() {
	<-c.start
}

// release allows all waiting requests to proceed exactly once.
func (c *blockingHTTPClient) release() {
	c.once.Do(func() {})
	select {
	case <-c.unblock:
		return
	default:
		close(c.unblock)
	}
}

// callCount reports how many times Do was invoked.
func (c *blockingHTTPClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}
