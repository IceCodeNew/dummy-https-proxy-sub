package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"dummy-https-proxy-sub/internal/proxy"
	"dummy-https-proxy-sub/internal/resolver"
)

func flagParser() (string, error) {
	portFlag := flag.String("port", "8000", "port to listen on")
	flag.Parse()

	port := *portFlag
	if _port := os.Getenv("PORT"); _port != "" {
		port = _port
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid port: %s", port)
	}
	return "0.0.0.0:" + port, nil
}

func main() {
	// Use standard library loggers; info -> stdout, errors -> stderr.
	infoLogger := log.New(os.Stdout, "INFO: ", log.LstdFlags)
	errorLogger := log.New(os.Stderr, "ERROR: ", log.LstdFlags)

	addr, err := flagParser()
	if err != nil {
		errorLogger.Fatal(err)
	}
	service := proxy.NewService(http.DefaultClient, resolver.NewNetResolver())
	handler := proxy.NewHandler(service, infoLogger, errorLogger)
	server := &http.Server{Addr: addr, Handler: handler}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			errorLogger.Fatalf("server shutdown error: %v", err)
		}
	}()

	infoLogger.Printf("listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errorLogger.Fatalf("server failed: %v", err)
	}
}
