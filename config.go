package main

import (
	"flag"
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

	flag.StringVar(&cfg.PelletmaticURL, "url", "http://localhost/pellematic.json", "Pellematic boiler JSON endpoint URL")
	flag.StringVar(&cfg.ListenAddress, "addr", ":8080", "HTTP server listen address")
	flag.StringVar(&cfg.MetricsPath, "path", "/metrics", "Metrics endpoint path")
	flag.DurationVar(&cfg.RefreshInterval, "interval", 30*time.Second, "Data refresh interval")
	logMode := flag.String("log", "development", "Log mode: development or production")
	flag.Parse()

	cfg.ProductionMode = *logMode == "production"

	return cfg
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
