package proxy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sync/singleflight"
)

// HTTPClient is the minimal subset of an http client we require.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service coordinates fetching upstream YAML and converting proxies into
// https://... lines which are then base64-encoded.
type Service struct {
	client HTTPClient
	group  singleflight.Group
}

// NewService constructs a Service.
func NewService(client HTTPClient) *Service {
	return &Service{client: client}
}

// Process fetches the YAML at targetURL and returns a single-line base64
// encoding of the newline-separated https proxy addresses.
func (s *Service) Process(ctx context.Context, targetURL string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("%w: service not initialized", ErrInvalidInput)
	}
	if s.client == nil {
		return "", fmt.Errorf("%w: HTTPClient not initialized", ErrInvalidInput)
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return "", fmt.Errorf("%w: empty target URL", ErrInvalidInput)
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("%w: target URL: %v", ErrInvalidInput, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: unsupported scheme %q", ErrInvalidInput, parsed.Scheme)
	}

	targetURL = parsed.String()
	resultCh := s.group.DoChan(targetURL, func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return "", fmt.Errorf("%w: craft request failed: %v", ErrInvalidInput, err)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("%w: fetch upstream failed: %v", ErrUpstream, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%w: upstream returned %d", ErrUpstream, resp.StatusCode)
		}
		proxies, bufSize, err := ParseProxiesFromReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrUpstream, err)
		}
		if len(proxies) == 0 {
			return "", fmt.Errorf("%w: no valid proxies found", ErrNoValidProxies)
		}
		return base64Encode(proxies, bufSize), nil
	})

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context canceled")
	case res := <-resultCh:
		if res.Err != nil {
			return "", res.Err
		}
		encoded, ok := res.Val.(string)
		if !ok {
			return "", fmt.Errorf("internal error: unexpected type %T of result", res.Val)
		}
		return encoded, nil
	}
}

func base64Encode(proxies []string, bufSize int) string {
	buf := make([]byte, 0, bufSize)
	for _, proxy := range proxies {
		buf = append(buf, proxy...)
		buf = append(buf, '\n')
	}
	return base64.StdEncoding.EncodeToString(buf)
}
