package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubProcessor struct {
	result     string
	err        error
	lastTarget string
}

func (s *stubProcessor) Process(ctx context.Context, targetURL string) (string, error) {
	s.lastTarget = targetURL
	return s.result, s.err
}

func TestHandlerSuccess(t *testing.T) {
	processor := &stubProcessor{result: "encoded-result"}
	infoLogger := log.New(io.Discard, "", 0)
	errorLogger := log.New(io.Discard, "", 0)
	handler := NewHandler(processor, infoLogger, errorLogger)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8000/https://example.com/path?group=gfw", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if processor.lastTarget != "https://example.com/path?group=gfw" {
		t.Fatalf("unexpected target passed to processor: %s", processor.lastTarget)
	}

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", got)
	}

	if body, _ := rec.Body.ReadString('\n'); body != processor.result {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestHandlerErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid", err: fmt.Errorf("%w: boom", ErrInvalidInput), wantStatus: http.StatusBadRequest},
		{name: "upstream", err: fmt.Errorf("%w: fail", ErrUpstream), wantStatus: http.StatusBadGateway},
		{name: "internal", err: errors.New("other"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &stubProcessor{err: tt.err}
			infoLogger := log.New(io.Discard, "", 0)
			errorLogger := log.New(io.Discard, "", 0)
			handler := NewHandler(processor, infoLogger, errorLogger)

			req := httptest.NewRequest(http.MethodGet, "http://localhost:8000/https://example.com", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status: want %d got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestHandlerUnavailable(t *testing.T) {
	handler := NewHandler(nil, log.New(io.Discard, "", 0), log.New(io.Discard, "", 0))
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8000/https://example.com", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d", rec.Result().StatusCode)
	}
}

func TestHandlerPreservesEscapedPath(t *testing.T) {
	processor := &stubProcessor{}
	infoLogger := log.New(io.Discard, "", 0)
	errorLogger := log.New(io.Discard, "", 0)
	handler := NewHandler(processor, infoLogger, errorLogger)

	// Path includes percent-encoding which must be preserved when reconstructing the target URL.
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8000/https://example.com/a%20b?x=1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	want := "https://example.com/a%20b?x=1"
	if processor.lastTarget != want {
		t.Fatalf("escaped path not preserved: want %q got %q", want, processor.lastTarget)
	}
}
