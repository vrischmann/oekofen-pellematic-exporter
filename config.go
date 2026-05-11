package main

import (
	"flag"
	"os"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	PelletmaticURL  string
	ListenAddress   string
	MetricsPath     string
	RefreshInterval time.Duration
	ProductionMode  bool
}

func parseConfig() *Config {
	cfg := &Config{}

	// Defaults: env vars override hardcoded defaults, CLI flags override env vars
	defaultURL := envOrDefault("PELLEMATIC_URL", "http://localhost/pellematic.json")
	defaultAddr := envOrDefault("PELLEMATIC_ADDR", ":48400")
	defaultPath := envOrDefault("PELLEMATIC_PATH", "/metrics")
	defaultInterval := envOrDefault("PELLEMATIC_INTERVAL", "30s")
	defaultLog := envOrDefault("PELLEMATIC_LOG", "development")

	flag.StringVar(&cfg.PelletmaticURL, "url", defaultURL, "Pellematic boiler JSON endpoint URL")
	flag.StringVar(&cfg.ListenAddress, "addr", defaultAddr, "HTTP server listen address")
	flag.StringVar(&cfg.MetricsPath, "path", defaultPath, "Metrics endpoint path")
	flag.DurationVar(&cfg.RefreshInterval, "interval", parseDuration(defaultInterval), "Data refresh interval")
	logMode := flag.String("log", defaultLog, "Log mode: development or production")
	flag.Parse()

	cfg.ProductionMode = *logMode == "production"

	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func setupLogger(productionMode bool) *zap.Logger {
	var logger *zap.Logger
	var err error

	if productionMode {
		logger, err = zap.NewProduction()
		if err != nil {
			panic(err)
		}
	} else {
		logger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	}

	return logger
}
