package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sync/singleflight"

	"dummy-https-proxy-sub/internal/yaml"
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
func (s *Service) Process(ctx context.Context, targetURL string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: service not initialized", ErrInvalidInput)
	}
	if s.client == nil {
		return nil, fmt.Errorf("%w: HTTPClient not initialized", ErrInvalidInput)
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, fmt.Errorf("%w: empty target URL", ErrInvalidInput)
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("%w: target URL: %v", ErrInvalidInput, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: unsupported scheme %q", ErrInvalidInput, parsed.Scheme)
	}

	targetURL = parsed.String()
	resultCh := s.group.DoChan(targetURL, func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: craft request failed: %v", ErrInvalidInput, err)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: fetch upstream failed: %v", ErrUpstream, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: upstream returned %d", ErrUpstream, resp.StatusCode)
		}
		lines, err := yaml.ParseProxiesFromReader(ctx, resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
		}
		return base64Encode(lines), nil
	})

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled")
	case res := <-resultCh:
		if res.Err != nil {
			return nil, res.Err
		}
		encoded, ok := res.Val.([]byte)
		if !ok {
			return nil, fmt.Errorf("internal error: unexpected type %T of result", res.Val)
		}
		return append(make([]byte, 0, len(encoded)), encoded...), nil
	}
}

func base64Encode(input []string) []byte {
	var buf bytes.Buffer
	for _, line := range input {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	result := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(result, buf.Bytes())
	return result
}
