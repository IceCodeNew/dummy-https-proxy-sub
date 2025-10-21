package yaml

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

// testResolver implements resolver.Resolver for deterministic tests.
type testResolver struct{ addrs map[string]net.IPAddr }

func (t *testResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if a, ok := t.addrs[host]; ok {
		return []net.IPAddr{a}, nil
	}
	return nil, fmt.Errorf("host not found: %s", host)
}

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

	r := &testResolver{addrs: map[string]net.IPAddr{"a.example": {IP: net.IPv4(1, 2, 3, 4)}}}
	lines, err := ParseProxiesFromReader(context.Background(), io.NopCloser(strings.NewReader(body)), r)
	if err != nil {
		t.Fatalf("ParseProxiesFromReader returned error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("want 1 transformed proxy, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "1.2.3.4:4433") {
		t.Fatalf("expected resolved ip and port in %q", lines[0])
	}
}

func TestParseProxiesFromReader_TooLarge(t *testing.T) {
	old := MaxYAMLBytes
	MaxYAMLBytes = 1
	defer func() { MaxYAMLBytes = old }()

	body := "proxies:\n- name: \"S\"\n  password: x\n"
	r2 := &testResolver{addrs: map[string]net.IPAddr{"a.example": {IP: net.IPv4(1, 2, 3, 4)}}}
	if _, err := ParseProxiesFromReader(context.Background(), strings.NewReader(body), r2); err == nil {
		t.Fatalf("expected error for severely truncated upstream, got nil")
	}
}
