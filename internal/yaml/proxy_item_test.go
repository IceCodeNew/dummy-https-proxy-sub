package yaml

import (
	"context"
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

	lines, err := ParseProxiesFromReader(context.Background(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatalf("ParseProxiesFromReader returned error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("want 1 transformed proxy, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "a.example:4433") {
		t.Fatalf("expected host %s not found in %s:", "a.example:4433", lines[0])
	}
}

func TestParseProxiesFromReader_TooLarge(t *testing.T) {
	old := MaxYAMLBytes
	MaxYAMLBytes = 1
	defer func() { MaxYAMLBytes = old }()

	body := "proxies:\n- name: \"S\"\n  password: x\n"
	if _, err := ParseProxiesFromReader(context.Background(), strings.NewReader(body)); err == nil {
		t.Fatalf("expected error for severely truncated upstream, got nil")
	}
}
