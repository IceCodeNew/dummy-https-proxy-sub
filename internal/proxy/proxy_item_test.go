package proxy

import (
	"io"
	"strings"
	"testing"
)

func TestParseProxiesFromReader_Success(t *testing.T) {
	body := `proxies:
- name: "Server-1"
  password: pass1
  port: 4433
  server: a.example
  sni: sni.example
  tls: true
  type: http
  username: admin
`

	proxies, _, err := ParseProxiesFromReader(io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatalf("ParseProxiesFromReader returned error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("want 1 transformed proxy, got %d", len(proxies))
	}

	expectedProxyHost := "a.example:4433"
	if !strings.Contains(proxies[0], expectedProxyHost) {
		t.Fatalf("expected proxy host %s not found, got %s", expectedProxyHost, proxies[0])
	}
}

func TestParseProxiesFromReader_TooLarge(t *testing.T) {
	old := maxYAMLBytes
	maxYAMLBytes = 1
	defer func() { maxYAMLBytes = old }()

	body := "proxies:\n- name: \"S\"\n  password: x\n"
	if _, _, err := ParseProxiesFromReader(strings.NewReader(body)); err == nil {
		t.Fatalf("expected error for severely truncated upstream, got nil")
	}
}
