package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"dummy-https-proxy-sub/internal/resolver"
	"dummy-https-proxy-sub/internal/yaml"
)

// HTTPClient is the minimal subset of an http client we require.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service coordinates fetching upstream YAML and converting proxies into
// https://... lines which are then base64-encoded.
type Service struct {
	client   HTTPClient
	resolver resolver.Resolver
}

// NewService constructs a Service.
func NewService(client HTTPClient, resolver resolver.Resolver) *Service {
	return &Service{client: client, resolver: resolver}
}

// Process fetches the YAML at targetURL and returns a single-line base64
// encoding of the newline-separated https proxy addresses.
func (s *Service) Process(ctx context.Context, targetURL string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: service not initialized", ErrInvalidInput)
	}
	if s.client == nil || s.resolver == nil {
		return nil, fmt.Errorf("%w: HTTPClient or resolver not initialized", ErrInvalidInput)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
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
	lines, err := yaml.ParseProxiesFromReader(ctx, resp.Body, s.resolver)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	return base64Encode(lines), nil
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
