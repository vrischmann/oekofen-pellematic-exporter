# Oekofen Pellematic Prometheus Exporter

A Prometheus exporter for Oekofen Pellematic pellet heating systems. It fetches JSON data from the boiler's built-in web interface and exposes it as Prometheus metrics for monitoring and alerting.

## Features

- Fetches metrics from Oekofen Pellematic boilers via their JSON endpoint
- Data-driven metric scaling using the `factor` field from the boiler's telemetry
- Human-readable metric descriptions from the boiler's own `text` metadata
- Graceful shutdown on SIGINT
- Development and production logging modes (structured JSON in production)
- Online/offline tracking: when the boiler is unreachable, stale metrics are removed and errors are counted
- Configuration via CLI flags or environment variables

## Supported Data Sections

The exporter processes all top-level sections from the Pellematic JSON endpoint (except `forecast`, which is skipped). Typical sections include:

- **system**: System-wide metrics (ambient temperature, errors, USB stick status, mode)
- **weather**: Weather data (temperature, clouds, forecast, thresholds)
- **hk1**: Heating circuit 1 (room/flow temperatures, pump status, state)
- **wireless1**: Wireless room sensor (temperature, humidity, battery, RSSI)
- **ww1**: Domestic hot water (temperatures, pump status, state)
- **pe1**: Pellet boiler (modulation, runtime, burner status, flue gas, storage fill)
- **error**: Error information

Additional sections (hk2, ww2, pe2, etc.) are automatically processed if present in the JSON response.

## Installation

### From Source

```bash
git clone ssh://git@git.rischmann.fr/vincent/oekofen-pellematic-exporter.git
cd oekofen-pellematic-exporter
go build -o oekofen-pellematic-exporter .
```

### Docker

A multi-arch (amd64/arm64) Docker image can be built and pushed using the justfile recipes:

```bash
just build           # Docker buildx
just build-podman    # Podman
```

## Usage

### Basic Usage

```bash
./oekofen-pellematic-exporter \
  -url http://192.168.1.100/pellematic_full.json \
  -addr :48400
```

### Configuration

All options can be set via CLI flags or environment variables. Environment variables are used as defaults; CLI flags take precedence.

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `-url` | `BOILER_URL` | `http://localhost/pellematic_full.json` | Pellematic boiler JSON endpoint URL |
| `-addr` | `LISTEN_ADDR` | `:48400` | HTTP server listen address |
| `-log` | `LOG_MODE` | `development` | Log mode: `development` or `production` |

### Docker

```bash
docker run -d \
  --name pellematic-exporter \
  -p 48400:48400 \
  -e BOILER_URL=http://192.168.1.100/pellematic_full.json \
  -e LOG_MODE=production \
  oekofen-pellematic-exporter
```

### Docker Compose

```yaml
services:
  pellematic-exporter:
    image: oekofen-pellematic-exporter
    container_name: pellematic-exporter
    ports:
      - "48400:48400"
    environment:
      - BOILER_URL=http://192.168.1.100/pellematic_full.json
      - LOG_MODE=production
    restart: unless-stopped
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'pellematic'
    static_configs:
      - targets: ['localhost:48400']
    scrape_interval: 30s
```

Match the Prometheus `scrape_interval` to the exporter's refresh interval (30s by default) to avoid stale or redundant scrapes.

## Metric Naming Convention

All metrics follow the pattern `pellematic_{section}_{field}`, where:

- **section** is the top-level JSON key (e.g., `system`, `hk1`, `pe1`)
- **field** is the nested key, lowercased, with the `L_` prefix stripped

For example, JSON field `pe1.L_temp_act` becomes metric `pellematic_pe1_temp_act`.

## JSON Format

When using the JSON endpoint (`pellematic_full.json`), each field value is an object containing rich metadata:

```json
"L_temp_act": {"val": 523, "unit": "°C", "factor": 0.1, "min": -32768, "max": 32767, "text": "PE T Chaudière"}
```

The exporter uses this metadata to:
- **Scale values** using the `factor` field (e.g., `523 * 0.1 = 52.3°C`)
- **Provide descriptions** via the `text` field (used as Prometheus metric help)
- **Skip unavailable data** when values match sentinel values (`32765`, `32767`, `-32768`)

String-valued fields (names, URLs, update timestamps) are automatically skipped.

## Available Metrics

### Scrape Metrics (always present)

| Metric | Type | Description |
|--------|------|-------------|
| `pellematic_scrape_errors_total` | counter | Total number of scrape errors from the Pellematic boiler |
| `pellematic_scrape_last_success_timestamp_seconds` | gauge | Unix timestamp of last successful scrape |

### Example Metrics by Section

Below are the most commonly used metrics. Metric help text comes directly from the boiler's `text` metadata.

#### System (`pellematic_system_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_system_ambient` | Ambient temperature (T extérieure) |
| `pellematic_system_errors` | Active errors (Défaut) |
| `pellematic_system_usb_stick` | USB stick connected |
| `pellematic_system_existing_boiler` | Measured temperature (T mes) |
| `pellematic_system_mode` | Mode (0=Off, 1=Auto, 2=DHW) |

#### Weather (`pellematic_weather_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_weather_temp` | Current outdoor temperature |
| `pellematic_weather_clouds` | Current cloud coverage |
| `pellematic_weather_forecast_temp` | Forecast average temperature |
| `pellematic_weather_forecast_clouds` | Forecast average cloudiness |
| `pellematic_weather_cloud_limit` | Weather threshold |
| `pellematic_weather_hysteresys` | Ambient temperature hysteresis for eco mode cutoff |
| `pellematic_weather_offtemp` | Outdoor cutoff temperature |
| `pellematic_weather_lead` | Anticipation duration (min) |
| `pellematic_weather_oekomode` | Eco mode (0=Off, 1=On) |

#### Heating Circuit 1 (`pellematic_hk1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_hk1_roomtemp_act` | Actual room temperature |
| `pellematic_hk1_roomtemp_set` | Set room temperature |
| `pellematic_hk1_flowtemp_act` | Actual flow temperature |
| `pellematic_hk1_flowtemp_set` | Set flow temperature |
| `pellematic_hk1_comfort` | Comfort temperature offset |
| `pellematic_hk1_state` | State code |
| `pellematic_hk1_pump` | Pump status (0=Off, 1=On) |
| `pellematic_hk1_statetext{component="..."}` | State text components (value=1.0) |
| `pellematic_hk1_temp_heat` | Comfort ambient temperature |
| `pellematic_hk1_temp_setback` | Setback ambient temperature |
| `pellematic_hk1_oekomode` | Eco mode |

#### Wireless Sensor (`pellematic_wireless1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_wireless1_wireless_temp` | Temperature |
| `pellematic_wireless1_wireless_hum` | Humidity |
| `pellematic_wireless1_wireless_rssi` | Signal strength (RSSI) |
| `pellematic_wireless1_wireless_batt` | Battery level (%) |

#### Domestic Hot Water (`pellematic_ww1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_ww1_temp_set` | DHW set temperature |
| `pellematic_ww1_ontemp_act` | On temperature (actual) |
| `pellematic_ww1_offtemp_act` | Off temperature (actual) |
| `pellematic_ww1_pump` | Pump status (0=Off, 1=On) |
| `pellematic_ww1_state` | State code |
| `pellematic_ww1_statetext{component="..."}` | State text components (value=1.0) |
| `pellematic_ww1_temp_max_set` | Max set temperature |
| `pellematic_ww1_smartstart` | Anticipated DHW charge (min) |
| `pellematic_ww1_use_boiler_heat` | Remaining energy used (0=Off, 1=On) |

#### Pellet Boiler (`pellematic_pe1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_pe1_temp_act` | Actual boiler temperature |
| `pellematic_pe1_temp_set` | Set temperature |
| `pellematic_pe1_frt_temp_act` | Flame temperature |
| `pellematic_pe1_modulation` | Modulation level (%) |
| `pellematic_pe1_runtime` | Total burner runtime (hours) |
| `pellematic_pe1_avg_runtime` | Average burner runtime (minutes) |
| `pellematic_pe1_runtimeburner` | Burner runtime |
| `pellematic_pe1_resttimeburner` | Burner rest time |
| `pellematic_pe1_starts` | Total burner starts |
| `pellematic_pe1_lowpressure` | Draft depression |
| `pellematic_pe1_lowpressure_set` | Set draft depression |
| `pellematic_pe1_fluegas` | Flue gas velocity |
| `pellematic_pe1_uw_speed` | UW speed (%) |
| `pellematic_pe1_uw` | UW speed |
| `pellematic_pe1_uw_release` | Limit temperature |
| `pellematic_pe1_storage_fill` | Pellets in silo (kg) |
| `pellematic_pe1_storage_min` | Pellet alert threshold (kg) |
| `pellematic_pe1_storage_max` | Max storage capacity (kg) |
| `pellematic_pe1_storage_popper` | Pellets in hopper (kg) |
| `pellematic_pe1_statetext{component="..."}` | State text components (value=1.0) |

Additional pe1 fields (`L_br`, `L_ak`, `L_not`, `L_stb`, `L_type`, `L_currentairflow`, `mode`) are also exposed.

## Metric Scaling

### JSON Format

When using the JSON endpoint, scaling is **data-driven**: each field carries a `factor` that the exporter applies automatically. For example:

| Field | Raw `val` | `factor` | Exported Value | Unit |
|-------|-----------|----------|----------------|------|
| `L_temp_act` | 523 | 0.1 | 52.3 | °C |
| `L_runtime` | 1672 | 1 | 1672 | h |
| `L_runtimeburner` | 0 | 0.01 | 0 | zs |
| `L_lowpressure` | 449 | 0.1 | 44.9 | EH |
| `L_starts` | 1605 | 1 | 1605 | - |

This eliminates the need for heuristic-based scaling and correctly handles all fields using the boiler's own `factor` metadata.

**Sentinel values** `32765`, `32767`, and `-32768` indicate unavailable data and are skipped entirely.

## Development

### Prerequisites

- Go 1.25 or later

### Building

```bash
go build -o oekofen-pellematic-exporter .
```

### Running Tests

```bash
go test -v -timeout=60s ./...
```

### Code Quality

```bash
go build ./...          # Compile check
gofmt -d -e             # Check formatting
gofmt -s -w .           # Fix formatting
staticcheck ./...       # Static analysis (use: just lint)
```

### Manual Testing

Use the provided test data for testing the collector:

```bash
# Serve the test data
python3 -m http.server 8000

# Run against the test data
go run . -url http://localhost:8000/testdata/pellematic.json
```

### Cross-Compilation

```bash
GOOS=linux GOARCH=amd64 go build -o oekofen-pellematic-exporter-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o oekofen-pellematic-exporter-linux-arm64 .
```

## Alerting Recommendations

Consider alerting on:
- `pellematic_scrape_errors_total` increasing (boiler unreachable)
- `pellematic_system_errors > 0` (active boiler errors)
- `pellematic_wireless1_wireless_batt < 20` (low battery on wireless sensor)
- Temperature deviations beyond expected ranges

## Troubleshooting

### Connection Errors

If you see "Failed to fetch data" errors:
- Verify the `-url` parameter (or `BOILER_URL` env var) points to the correct Pellematic JSON endpoint
- Check network connectivity to the boiler
- Ensure the boiler's web interface is accessible (try opening the URL in a browser)

### No Metrics Exposed

If no section metrics appear:
- Check the exporter logs for errors (use `-log=development` for verbose output)
- The boiler may be offline; only the scrape error/success metrics will be present until connectivity is restored
- Ensure the boiler returns valid JSON at the configured URL

### Incorrect Values

- Values are scaled by the `factor` field provided by the boiler itself
- Sentinel values (32765, 32767, -32768) are automatically filtered out

## License

MIT
