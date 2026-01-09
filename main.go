package main

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	cfg := parseConfig()
	logger := setupLogger(cfg.ProductionMode)
	logger.Info("Starting Pellematic exporter",
		zap.String("url", cfg.PelletmaticURL),
		zap.Duration("interval", cfg.RefreshInterval),
	)

	collector := NewCollector(cfg, logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	ctx := context.Background()
	go collector.Start(ctx)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	http.HandleFunc(cfg.MetricsPath, func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: nil,
	}

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down HTTP server")
		shutdownCtx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(shutdownCtx)
	}()

	logger.Info("HTTP server listening", zap.String("addr", cfg.ListenAddress))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
