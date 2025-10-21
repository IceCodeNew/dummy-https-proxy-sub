package proxy

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
)

// Processor captures the behaviour required by the HTTP handler.
type Processor interface {
	Process(ctx context.Context, targetURL string) ([]byte, error)
}

// Handler routes incoming HTTP requests through the provided Processor.
type Handler struct {
	processor   Processor
	infoLogger  *log.Logger
	errorLogger *log.Logger
}

// NewHandler builds a Handler that delegates to the provided Processor and logger.
func NewHandler(processor Processor, infoLogger, errorLogger *log.Logger) *Handler {
	if processor == nil || infoLogger == nil || errorLogger == nil {
		return nil
	}
	return &Handler{processor: processor, infoLogger: infoLogger, errorLogger: errorLogger}
}

// ServeHTTP extracts the target URL from the request path, processes it, and delivers the base64 payload.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.processor == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Trimming the leading slash yields the embedded target URL.
	target := strings.TrimPrefix(r.URL.EscapedPath(), "/")
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	encoded, err := h.processor.Process(r.Context(), target)
	if err != nil {
		status := statusFromError(err)
		message := http.StatusText(status)
		if message == "" {
			message = "unexpected error"
		}
		http.Error(w, message, status)

		if h.errorLogger != nil {
			h.errorLogger.Printf("request failed: target=%q status=%d error=%v", target, status, err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(encoded); err != nil && h.errorLogger != nil {
		h.errorLogger.Printf("failed to write response: %v", err)
	}
}

func statusFromError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrUpstream):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
