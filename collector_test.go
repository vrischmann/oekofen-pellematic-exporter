package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newTestCollector creates a Collector backed by an httptest.Server using the
// given handler. The server is automatically closed when the test finishes.
func newTestCollector(t *testing.T, handler http.HandlerFunc) (*Collector, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(func() { ts.Close() })

	cfg := &Config{
		BoilerURL:     ts.URL,
		ListenAddress: ":0",
	}
	logger := zaptest.NewLogger(t)
	collector := NewCollector(cfg, logger)
	return collector, ts
}

// gatherMetrics collects all metrics from the collector and returns a flat map
// of "metric_name{label1=val1,...}" to the metric value.
func gatherMetrics(c *Collector) map[string]float64 {
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	families, _ := reg.Gather()

	result := make(map[string]float64)
	for _, fam := range families {
		name := fam.GetName()
		for _, m := range fam.GetMetric() {
			key := name
			labels := m.GetLabel()
			if len(labels) > 0 {
				parts := make([]string, len(labels))
				for i, l := range labels {
					parts[i] = fmt.Sprintf("%s=%q", l.GetName(), l.GetValue())
				}
				key = fmt.Sprintf("%s{%s}", name, strings.Join(parts, ","))
			}
			if u := m.GetUntyped(); u != nil {
				result[key] = u.GetValue()
			} else if g := m.GetGauge(); g != nil {
				result[key] = g.GetValue()
			} else if ct := m.GetCounter(); ct != nil {
				result[key] = ct.GetValue()
			}
		}
	}
	return result
}

// requireMetric asserts that a metric exists with approximately the expected value.
func requireMetric(t *testing.T, metrics map[string]float64, name string, want float64) {
	t.Helper()
	got, ok := metrics[name]
	require.True(t, ok, "metric %q not found in collected metrics", name)
	require.InDelta(t, want, got, 1e-9, "metric %q", name)
}

// requireNoMetric asserts that a metric does not exist.
func requireNoMetric(t *testing.T, metrics map[string]float64, name string) {
	t.Helper()
	_, ok := metrics[name]
	require.False(t, ok, "expected metric %q to be absent, but it was present", name)
}

// --- Collector integration tests ---

func TestCollector_UpdateMetrics_Success(t *testing.T) {
	data, err := os.ReadFile("testdata/pellematic.json")
	require.NoError(t, err)

	collector, _ := newTestCollector(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))

	collector.updateMetrics()
	require.True(t, collector.isOnline, "expected collector to be online after successful fetch")

	metrics := gatherMetrics(collector)

	// --- Factor scaling (full format) ---
	// system.L_ambient: val=107, factor=0.1 -> 10.7
	requireMetric(t, metrics, "pellematic_system_ambient", 10.7)
	// system.L_errors: val=0, factor=1 -> 0
	requireMetric(t, metrics, "pellematic_system_errors", 0)

	// weather.L_temp: val=80, factor=0.1 -> 8.0
	requireMetric(t, metrics, "pellematic_weather_temp", 8.0)
	// weather.L_clouds: val=75, factor=1 -> 75
	requireMetric(t, metrics, "pellematic_weather_clouds", 75)
	// weather.hysteresys: val=-40, factor=0.1 -> -4.0
	requireMetric(t, metrics, "pellematic_weather_hysteresys", -4.0)

	// hk1.L_roomtemp_act: val=195, factor=0.1 -> 19.5
	requireMetric(t, metrics, "pellematic_hk1_roomtemp_act", 19.5)
	// hk1.L_flowtemp_act: val=233, factor=0.1 -> 23.3
	requireMetric(t, metrics, "pellematic_hk1_flowtemp_act", 23.3)

	// wireless1.L_wireless_temp: val=195, factor=0.1 -> 19.5
	requireMetric(t, metrics, "pellematic_wireless1_wireless_temp", 19.5)
	// wireless1.L_wireless_hum: val=471, factor=0.1 -> 47.1
	requireMetric(t, metrics, "pellematic_wireless1_wireless_hum", 47.1)
	// wireless1.L_wireless_rssi: val=-56, factor=1 -> -56
	requireMetric(t, metrics, "pellematic_wireless1_wireless_rssi", -56)

	// pe1.L_temp_act: val=523, factor=0.1 -> 52.3
	requireMetric(t, metrics, "pellematic_pe1_temp_act", 52.3)
	// pe1.L_runtime: val=1672, factor=1 -> 1672
	requireMetric(t, metrics, "pellematic_pe1_runtime", 1672)
	// pe1.L_starts: val=1605, factor=1 -> 1605
	requireMetric(t, metrics, "pellematic_pe1_starts", 1605)
	// pe1.L_storage_fill: val=6000, factor=1 -> 6000
	requireMetric(t, metrics, "pellematic_pe1_storage_fill", 6000)
	// pe1.L_lowpressure: val=449, factor=0.1 -> 44.9
	requireMetric(t, metrics, "pellematic_pe1_lowpressure", 44.9)

	// ww1.L_temp_set: val=80, factor=0.1 -> 8.0
	requireMetric(t, metrics, "pellematic_ww1_temp_set", 8.0)
	// ww1.L_ontemp_act: val=504, factor=0.1 -> 50.4
	requireMetric(t, metrics, "pellematic_ww1_ontemp_act", 50.4)

	// --- Sentinel values are skipped ---
	requireNoMetric(t, metrics, "pellematic_pe1_ext_temp")               // val=-32768
	requireNoMetric(t, metrics, "pellematic_pe1_storage_fill_today")     // val=32765
	requireNoMetric(t, metrics, "pellematic_pe1_storage_fill_yesterday") // val=32765

	// --- String-valued fields are skipped ---
	requireNoMetric(t, metrics, "pellematic_weather_source")
	requireNoMetric(t, metrics, "pellematic_weather_location")
	requireNoMetric(t, metrics, "pellematic_wireless1_wireless_name")
	requireNoMetric(t, metrics, "pellematic_wireless1_wireless_update")
	requireNoMetric(t, metrics, "pellematic_hk1_name")
	requireNoMetric(t, metrics, "pellematic_ww1_name")

	// --- *_info keys are skipped ---
	requireNoMetric(t, metrics, "pellematic_system_system_info")
	requireNoMetric(t, metrics, "pellematic_weather_weather_info")
	requireNoMetric(t, metrics, "pellematic_hk1_hk_info")
	requireNoMetric(t, metrics, "pellematic_pe1_pe_info")

	// --- forecast section is entirely skipped ---
	for key := range metrics {
		require.False(t, strings.HasPrefix(key, "pellematic_forecast"),
			"expected no forecast metrics, found %q", key)
	}
}

func TestCollector_Statetext(t *testing.T) {
	data, err := os.ReadFile("testdata/pellematic.json")
	require.NoError(t, err)

	collector, _ := newTestCollector(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))

	collector.updateMetrics()
	metrics := gatherMetrics(collector)

	// hk1 statetext: "Mode réduit actif|T ext supérieure à la limite de réduit"
	requireMetric(t, metrics, `pellematic_hk1_statetext{component="Mode réduit actif"}`, 1.0)
	requireMetric(t, metrics, `pellematic_hk1_statetext{component="T ext supérieure à la limite de réduit"}`, 1.0)

	// pe1 statetext: "Arrêt"
	requireMetric(t, metrics, `pellematic_pe1_statetext{component="Arrêt"}`, 1.0)

	// ww1 statetext: "t hors prog horaire|Demande marche off"
	requireMetric(t, metrics, `pellematic_ww1_statetext{component="t hors prog horaire"}`, 1.0)
	requireMetric(t, metrics, `pellematic_ww1_statetext{component="Demande marche off"}`, 1.0)
}

func TestCollector_FetchError_ServerError(t *testing.T) {
	collector, _ := newTestCollector(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	collector.updateMetrics()
	require.False(t, collector.isOnline, "expected collector to be offline after server error")
}

func TestCollector_FetchError_InvalidJSON(t *testing.T) {
	collector, _ := newTestCollector(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))

	collector.updateMetrics()
	require.False(t, collector.isOnline, "expected collector to be offline after invalid JSON response")
}

func TestCollector_FetchError_ConnectionRefused(t *testing.T) {
	cfg := &Config{
		BoilerURL:     "http://127.0.0.1:1", // port 1 should always refuse
		ListenAddress: ":0",
	}
	logger := zaptest.NewLogger(t)
	collector := NewCollector(cfg, logger)

	collector.updateMetrics()
	require.False(t, collector.isOnline, "expected collector to be offline after connection refused")
}

func TestCollector_EmptySection(t *testing.T) {
	// The testdata has an empty "error" section; verify it doesn't panic.
	data, err := os.ReadFile("testdata/pellematic.json")
	require.NoError(t, err)

	collector, _ := newTestCollector(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))

	collector.updateMetrics()
	require.True(t, collector.isOnline, "expected collector to be online")

	// Should not produce any "pellematic_error_*" metrics since the section is empty
	metrics := gatherMetrics(collector)
	for key := range metrics {
		require.False(t, strings.HasPrefix(key, "pellematic_error"),
			"expected no error-section metrics, found %q", key)
	}
}

// --- Unit tests for helper functions ---

func TestIsSentinelValue(t *testing.T) {
	tests := []struct {
		value float64
		want  bool
	}{
		{32765, true},
		{32767, true},
		{-32768, true},
		{0, false},
		{100, false},
		{-1, false},
		{32766, false},
		{-32767, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("val_%v", tt.value), func(t *testing.T) {
			got := isSentinelValue(tt.value)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{float64(3.14), 3.14, true},
		{int(42), 42.0, true},
		{int64(100), 100.0, true},
		{"string", 0, false},
		{nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T(%v)", tt.input, tt.input), func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCleanLabelName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"L_temp_act", "temp_act"},
		{"L_errors", "errors"},
		{"temp_act", "temp_act"},
		{"L_ambient", "ambient"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanLabelName(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildMetricName(t *testing.T) {
	tests := []struct {
		prefix, component, field string
		want                     string
	}{
		{"system", "", "L_ambient", "pellematic_system_ambient"},
		{"hk1", "", "L_roomtemp_act", "pellematic_hk1_roomtemp_act"},
		{"pe1", "", "L_temp_act", "pellematic_pe1_temp_act"},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%s/%s", tt.prefix, tt.field)
		t.Run(name, func(t *testing.T) {
			got := buildMetricName(tt.prefix, tt.component, tt.field)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestProcessValue(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &Config{BoilerURL: "http://localhost", ListenAddress: ":0"}
	c := NewCollector(cfg, logger)

	tests := []struct {
		field string
		value any
		want  float64
	}{
		// Temperature: divide by 10
		{"L_temp_act", float64(523), 52.3},
		{"L_roomtemp_act", float64(195), 19.5},
		// Runtime: multiply by 3600
		{"L_runtime", float64(1672), 1672 * 3600.0},
		// Starts: no scaling
		{"L_starts", float64(1605), 1605},
		// Humidity: divide by 10
		{"L_wireless_hum", float64(471), 47.1},
		// Modulation: no scaling
		{"L_modulation", float64(50), 50},
		// Storage fill: no scaling
		{"L_storage_fill", float64(6000), 6000},
		// Default: no scaling
		{"L_some_field", float64(42), 42},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := c.processValue(tt.field, tt.value)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestProcessStateText(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &Config{BoilerURL: "http://localhost", ListenAddress: ":0"}
	c := NewCollector(cfg, logger)

	tests := []struct {
		name     string
		prefix   string
		input    string
		wantLen  int
		wantText []string
	}{
		{
			name:     "single component",
			prefix:   "pe1",
			input:    "Arrêt",
			wantLen:  1,
			wantText: []string{"Arrêt"},
		},
		{
			name:     "two components",
			prefix:   "hk1",
			input:    "Mode réduit actif|T ext supérieure à la limite de réduit",
			wantLen:  2,
			wantText: []string{"Mode réduit actif", "T ext supérieure à la limite de réduit"},
		},
		{
			name:     "empty string",
			prefix:   "hk1",
			input:    "",
			wantLen:  0,
			wantText: nil,
		},
		{
			name:     "trailing pipe",
			prefix:   "hk1",
			input:    "comp1|",
			wantLen:  1,
			wantText: []string{"comp1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := c.processStateText(tt.prefix, tt.input)
			require.Len(t, metrics, tt.wantLen)
		})
	}
}
