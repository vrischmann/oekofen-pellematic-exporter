package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	cfg := parseConfig()

	logger := setupLogger(cfg.ProductionMode)
	logger.Info(
		"Starting Pellematic exporter",
		zap.String("url", cfg.BoilerURL),
	)

	collector := NewCollector(cfg, logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go collector.Start(ctx)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: nil,
	}

	errCh := make(chan error)
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", cfg.ListenAddress))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Fatal("Failed to start server", zap.Error(err))

	case <-ctx.Done():
		logger.Info("Shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server.Shutdown(shutdownCtx)
	}
}
