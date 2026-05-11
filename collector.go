package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/charmap"
)

type PellematicData map[string]interface{}

type Collector struct {
	config       *Config
	logger       *zap.Logger
	client       *http.Client
	metrics      map[string][]prometheus.Metric
	metricsMutex sync.RWMutex
	isOnline     bool
	onlineMutex  sync.RWMutex
}

var (
	scrapeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pellematic_scrape_errors_total",
		Help: "Total number of scrape errors from Pellematic boiler",
	})
	scrapeLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pellematic_scrape_last_success_timestamp_seconds",
		Help: "Unix timestamp of last successful scrape",
	})
)

func NewCollector(cfg *Config, logger *zap.Logger) *Collector {
	return &Collector{
		config:   cfg,
		logger:   logger,
		client:   &http.Client{Timeout: 3 * time.Second},
		metrics:  make(map[string][]prometheus.Metric),
		isOnline: false,
	}
}

func (c *Collector) fetchData() (*PellematicData, error) {
	req, err := http.NewRequest("GET", c.config.PelletmaticURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	decoder := charmap.ISO8859_1.NewDecoder().Reader(resp.Body)
	body, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	bodyStr := string(body)
	bodyStr = strings.ReplaceAll(bodyStr, `L_statetext:`, `L_statetext":`)

	var data PellematicData
	if err := json.Unmarshal([]byte(bodyStr), &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &data, nil
}

func (c *Collector) processValue(field string, value interface{}) float64 {
	var floatValue float64

	switch v := value.(type) {
	case float64:
		floatValue = v
	case int:
		floatValue = float64(v)
	case int64:
		floatValue = float64(v)
	default:
		return 0
	}

	if strings.Contains(field, "temp") {
		return floatValue / 10.0
	}

	if strings.Contains(field, "runtime") {
		return floatValue * 3600.0
	}

	if strings.Contains(field, "avg_runtime") {
		return floatValue * 60.0
	}

	if strings.Contains(field, "runtimeburner") || strings.Contains(field, "resttimeburner") {
		return floatValue
	}

	if strings.Contains(field, "starts") {
		return floatValue
	}

	if strings.Contains(field, "humidity") || strings.Contains(field, "hum") {
		return floatValue / 10.0
	}

	if strings.Contains(field, "_uw") || strings.Contains(field, "_fluegas") || strings.Contains(field, "modulation") || strings.Contains(field, "lowpressure") {
		return floatValue
	}

	if strings.Contains(field, "storage_fill") || strings.Contains(field, "pellets") {
		return floatValue
	}

	return floatValue
}

func (c *Collector) processStateText(prefix string, statetext string) []prometheus.Metric {
	var metrics []prometheus.Metric

	components := strings.Split(statetext, "|")
	for _, comp := range components {
		comp = strings.TrimSpace(comp)
		if comp == "" {
			continue
		}

		metricName := fmt.Sprintf("pellematic_%s_statetext", prefix)
		desc := prometheus.NewDesc(metricName, fmt.Sprintf("%s state component", prefix), []string{"component"}, nil)
		metric := prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, 1.0, comp)
		metrics = append(metrics, metric)
	}

	return metrics
}

func (c *Collector) updateMetrics() {
	newData, err := c.fetchData()
	if err != nil {
		c.logger.Warn("Failed to fetch data", zap.Error(err))
		scrapeErrors.Inc()
		c.setOnline(false)
		return
	}

	c.setOnline(true)
	scrapeLastSuccess.Set(float64(time.Now().Unix()))

	c.logger.Debug("Successfully fetched data")

	c.metricsMutex.Lock()
	defer c.metricsMutex.Unlock()

	c.metrics = make(map[string][]prometheus.Metric)

	for sectionName, sectionData := range *newData {
		if sectionName == "forecast" {
			continue
		}

		switch v := sectionData.(type) {
		case map[string]interface{}:
			c.processSection(sectionName, v)
		}
	}
}

func (c *Collector) processSection(prefix string, section map[string]interface{}) {
	for key, value := range section {
		if key == "L_statetext" {
			if textValue, ok := value.(string); ok {
				metrics := c.processStateText(prefix, textValue)
				c.metrics[fmt.Sprintf("%s:%s", prefix, key)] = metrics
			}
			continue
		}

		metricName := buildMetricName(prefix, "", key)

		intValue, ok := value.(int)
		if ok {
			if intValue == 32765 || intValue == 32767 || intValue == -32768 {
				c.logger.Debug("Skipping sentinel value", zap.String("field", key))
				continue
			}
		}

		scaledValue := c.processValue(key, value)

		desc := prometheus.NewDesc(metricName, fmt.Sprintf("%s metric", key), nil, nil)
		metric := prometheus.MustNewConstMetric(desc, prometheus.UntypedValue, scaledValue)
		c.metrics[metricName] = []prometheus.Metric{metric}
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	scrapeErrors.Describe(ch)
	scrapeLastSuccess.Describe(ch)

	c.onlineMutex.RLock()
	online := c.isOnline
	c.onlineMutex.RUnlock()

	if !online {
		return
	}

	c.metricsMutex.RLock()
	defer c.metricsMutex.RUnlock()

	for _, metrics := range c.metrics {
		for _, m := range metrics {
			ch <- m.Desc()
		}
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	scrapeErrors.Collect(ch)
	scrapeLastSuccess.Collect(ch)

	c.onlineMutex.RLock()
	online := c.isOnline
	c.onlineMutex.RUnlock()

	if !online {
		return
	}

	c.metricsMutex.RLock()
	defer c.metricsMutex.RUnlock()

	for _, metrics := range c.metrics {
		for _, m := range metrics {
			ch <- m
		}
	}
}

func (c *Collector) setOnline(online bool) {
	c.onlineMutex.Lock()
	defer c.onlineMutex.Unlock()
	c.isOnline = online
}

func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.config.RefreshInterval)
	defer ticker.Stop()

	c.updateMetrics()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Collector shutting down")
			return
		case <-ticker.C:
			c.updateMetrics()
		}
	}
}
