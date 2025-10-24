package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
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

	service := NewService(client)
	encoded, err := service.Process(context.Background(), "https://source.example/config")
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	proxy1, proxy2 := "https://admin:%3Credacted%3E@1.server.xyz:4433?sni=pku.speedtest.ooklaserver.wallesspku.space#Server-1\n",
		"https://admin:%3Credacted%3E@2.server.xyz:4444?sni=pku.speedtest.ooklaserver.wallesspku.space#Server-2\n"
	expected := base64.StdEncoding.EncodeToString([]byte(proxy1 + proxy2))

	if encoded != expected {
		t.Fatalf("unexpected encoded output: %s", encoded)
	}

	if client.lastURL != "https://source.example/config" {
		t.Fatalf("unexpected upstream url. want https://source.example/config got %s", client.lastURL)
	}
}

func TestServiceProcessInvalidScheme(t *testing.T) {
	client := &fakeHTTPClient{}
	service := NewService(client)
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
	oldMaxYAMLBytes := maxYAMLBytes
	maxYAMLBytes = 1
	defer func() { maxYAMLBytes = oldMaxYAMLBytes }()

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

	service := NewService(client)
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

func TestServiceProcessContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &fakeHTTPClient{err: context.Canceled}
	service := NewService(client)

	if _, err := service.Process(ctx, "https://source.example/config"); err == nil {
		t.Fatalf("expected cancellation error")
	}
}

func TestServiceProcessSingleFlight(t *testing.T) {
	t.Parallel()

	yamlBody := `proxies:
- name: "Server-1"
  password: <redacted>
  port: 4433
  server: 1.server.xyz
  sni: sni.example
  tls: true
  type: http
  username: admin
`

	client := newBlockingHTTPClient(yamlBody)
	service := NewService(client)

	expected := base64Encode([]string{
		"https://admin:%3Credacted%3E@1.server.xyz:4433?sni=sni.example#Server-1",
	}, 72)

	const workers = 4
	start := make(chan struct{})
	ready := make(chan struct{}, workers)

	var (
		wg      sync.WaitGroup
		results = make([]string, workers)
		errs    = make([]error, workers)
	)

	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// block this goroutine, makes sure that all workers launch their HTTP call together
			// once the main goroutine closed the start channel.
			<-start

			// signals current worker is about to call the service.Process function.
			// when the main goroutine knows all workers are ready, it calls the release function,
			// letting all workers launch their HTTP call at the same time.
			ready <- struct{}{}
			res, err := service.Process(context.Background(), "https://source.example/config")
			errs[idx], results[idx] = err, res
		}(i)
	}

	close(start)
	for range workers {
		<-ready
	}
	client.waitUntilStarted()
	client.release()
	wg.Wait()

	if got := client.callCount(); got != 1 {
		t.Fatalf("expected exactly one upstream request, got %d", got)
	}

	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("worker %d returned error: %v", i, errs[i])
		}
		if results[i] != expected {
			t.Fatalf("worker %d unexpected result: want %s got %s", i, expected, results[i])
		}
	}
}
