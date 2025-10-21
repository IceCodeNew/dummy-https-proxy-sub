package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"dummy-https-proxy-sub/internal/yaml"
)

type fakeHTTPClient struct {
	response *http.Response
	err      error
	lastURL  string
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.lastURL = req.URL.String()
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

type fakeResolver struct {
	addrs map[string][]net.IPAddr
}

func (f *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if addrs, ok := f.addrs[host]; ok {
		return addrs, nil
	}
	return nil, errors.New("host not found")
}

func TestServiceProcessSuccess(t *testing.T) {
	yamlBody := `proxies:
- name: "Server-1"
  password: <redacted>
  port: 4433
  server: 1.server.xyz
  sni: pku.speedtest.ooklaserver.wallesspku.space
  tls: true
  type: http
  username: admin
- name: "Server-2"
  password: <redacted>
  port: 4444
  server: 2.server.xyz
  sni: pku.speedtest.ooklaserver.wallesspku.space
  tls: true
  type: http
  username: admin
`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(yamlBody)),
		Header:     make(http.Header),
	}

	client := &fakeHTTPClient{response: resp}
	resolver := &fakeResolver{
		addrs: map[string][]net.IPAddr{
			"1.server.xyz": {{IP: net.IPv4(12, 34, 56, 78)}},
			"2.server.xyz": {{IP: net.IPv4(23, 45, 67, 89)}},
		},
	}

	service := NewService(client, resolver)
	encoded, err := service.Process(context.Background(), "https://source.example/config")
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	expectedLines := []string{
		"https://admin:%3Credacted%3E@12.34.56.78:4433?sni=pku.speedtest.ooklaserver.wallesspku.space#Server-1",
		"https://admin:%3Credacted%3E@23.45.67.89:4444?sni=pku.speedtest.ooklaserver.wallesspku.space#Server-2",
	}
	expected := base64Encode(expectedLines)

	if !bytes.Equal(encoded, expected) {
		t.Fatalf("unexpected encoded output. want %s got %s", expected, encoded)
	}

	if client.lastURL != "https://source.example/config" {
		t.Fatalf("unexpected upstream url. want https://source.example/config got %s", client.lastURL)
	}
}

func TestServiceProcessInvalidScheme(t *testing.T) {
	client := &fakeHTTPClient{}
	resolver := &fakeResolver{}
	service := NewService(client, resolver)
	_, err := service.Process(context.Background(), "ftp://invalid")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestServiceProcessTooLarge(t *testing.T) {
	// temporarily shrink the limit so the test runs quickly
	old := yaml.MaxYAMLBytes
	yaml.MaxYAMLBytes = 1
	defer func() { yaml.MaxYAMLBytes = old }()

	// build a body larger than the temporary limit
	var sb strings.Builder
	sb.WriteString("proxies:\n")
	for i := 0; sb.Len() < 1024; i++ {
		sb.WriteString(fmt.Sprintf("- name: \"S%d\"\n  password: <redacted>\n  port: 4433\n  server: 1.server.xyz\n  sni: sni.example\n  tls: true\n  type: http\n  username: admin\n", i))
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sb.String())),
		Header:     make(http.Header),
	}

	client := &fakeHTTPClient{response: resp}
	resolver := &fakeResolver{
		addrs: map[string][]net.IPAddr{
			"1.server.xyz": {{IP: net.IPv4(1, 2, 3, 4)}},
		},
	}

	service := NewService(client, resolver)
	_, err := service.Process(context.Background(), "https://source.example/config")
	if err == nil {
		t.Fatalf("expected error for truncated upstream")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream for truncated upstream, got %v", err)
	}
	if !strings.Contains(err.Error(), "read proxies") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
