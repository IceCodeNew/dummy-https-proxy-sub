package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	lg "dummy-https-proxy-sub/internal/logger"
	"dummy-https-proxy-sub/internal/proxy"
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
	addr, err := flagParser()
	if err != nil {
		lg.ErrorLogger.Fatal(err)
	}
	handler := proxy.NewHandler(proxy.NewService(http.DefaultClient))
	server := &http.Server{Addr: addr, Handler: handler}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			lg.ErrorLogger.Fatalf("server shutdown error: %v", err)
		}
	}()

	lg.InfoLogger.Printf("listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		lg.ErrorLogger.Fatalf("server failed: %v", err)
	}
}
