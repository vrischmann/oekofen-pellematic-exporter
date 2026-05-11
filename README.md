# Oekofen Pellematic Prometheus Exporter

A Prometheus exporter for Oekofen Pellematic pellet heating systems. It fetches JSON data from the boiler's built-in web interface and exposes it as Prometheus metrics for monitoring and alerting.

## Features

- Fetches metrics from Oekofen Pellematic boilers via their JSON endpoint
- Automatic metric scaling for temperatures, runtimes, and humidity
- Graceful shutdown on SIGINT
- Development and production logging modes (structured JSON in production)
- Online/offline tracking: when the boiler is unreachable, stale metrics are removed and errors are counted

## Supported Data Sections

The exporter processes all top-level sections from the Pellematic JSON endpoint (except `forecast`, which is skipped). Typical sections include:

- **system**: System-wide metrics (ambient, errors, USB stick status, mode)
- **weather**: Weather data (temperature, clouds, forecast, location)
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

## Usage

### Basic Usage

```bash
./oekofen-pellematic-exporter \
  -url http://192.168.1.100/pellematic.json \
  -addr :8080 \
  -path /metrics \
  -interval 30s
```

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost/pellematic.json` | Pellematic boiler JSON endpoint URL |
| `-addr` | `:8080` | HTTP server listen address |
| `-path` | `/metrics` | Metrics endpoint path |
| `-interval` | `30s` | Data refresh interval |
| `-log` | `development` | Log mode: `development` or `production` |

### Docker

```bash
docker run -d \
  --name pellematic-exporter \
  -p 8080:8080 \
  oekofen-pellematic-exporter \
  -url http://192.168.1.100/pellematic.json \
  -interval 30s \
  -log production
```

### Docker Compose

```yaml
services:
  pellematic-exporter:
    image: oekofen-pellematic-exporter
    container_name: pellematic-exporter
    ports:
      - "8080:8080"
    command:
      - "-url=http://192.168.1.100/pellematic.json"
      - "-addr=:8080"
      - "-interval=30s"
      - "-log=production"
    restart: unless-stopped
```

## Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'pellematic'
    static_configs:
      - targets: ['localhost:8080']
    scrape_interval: 30s
```

Match the Prometheus `scrape_interval` to the exporter's `-interval` flag to avoid stale or redundant scrapes.

## Metric Naming Convention

All metrics follow the pattern `pellematic_{section}_{field}`, where:

- **section** is the top-level JSON key (e.g., `system`, `hk1`, `pe1`)
- **field** is the nested key, lowercased, with the `L_` prefix stripped

For example, JSON field `pe1.L_temp_act` becomes metric `pellematic_pe1_temp_act`.

## Available Metrics

### Scrape Metrics (always present)

| Metric | Type | Description |
|--------|------|-------------|
| `pellematic_scrape_errors_total` | counter | Total number of scrape errors from the Pellematic boiler |
| `pellematic_scrape_last_success_timestamp_seconds` | gauge | Unix timestamp of last successful scrape |

### Example Metrics by Section

Below are the most commonly used metrics. The exporter automatically discovers all numeric fields, so your boiler may expose additional metrics depending on its model and firmware.

#### System (`pellematic_system_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_system_l_ambient` | Ambient value |
| `pellematic_system_l_errors` | Number of active errors |
| `pellematic_system_l_usb_stick` | USB stick status |
| `pellematic_system_l_existing_boiler` | Existing boiler status |
| `pellematic_system_mode` | System mode |

#### Weather (`pellematic_weather_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_weather_l_temp` | Outdoor temperature (scaled, see below) |
| `pellematic_weather_l_clouds` | Cloud coverage |
| `pellematic_weather_l_forecast_temp` | Forecast temperature (scaled) |
| `pellematic_weather_l_forecast_clouds` | Forecast cloud coverage |
| `pellematic_weather_l_starttime` | Start time |
| `pellematic_weather_l_endtime` | End time |

Additional weather fields (`cloud_limit`, `hysteresys`, `offtemp`, `lead`, `oekomode`) are also exposed.

#### Heating Circuit 1 (`pellematic_hk1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_hk1_l_roomtemp_act` | Actual room temperature (scaled) |
| `pellematic_hk1_l_roomtemp_set` | Set room temperature (scaled) |
| `pellematic_hk1_l_flowtemp_act` | Actual flow temperature (scaled) |
| `pellematic_hk1_l_flowtemp_set` | Set flow temperature (scaled) |
| `pellematic_hk1_l_comfort` | Comfort mode status |
| `pellematic_hk1_l_state` | Heating circuit state code |
| `pellematic_hk1_l_pump` | Pump status (0=off, 1=on) |
| `pellematic_hk1_statetext{component="..."}` | State text components (value=1.0) |

#### Wireless Sensor (`pellematic_wireless1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_wireless1_l_wireless_temp` | Sensor temperature (scaled) |
| `pellematic_wireless1_l_wireless_hum` | Sensor humidity (scaled) |
| `pellematic_wireless1_l_wireless_rssi` | Signal strength |
| `pellematic_wireless1_l_wireless_batt` | Battery level |

#### Domestic Hot Water (`pellematic_ww1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_ww1_l_temp_set` | Set temperature (scaled) |
| `pellematic_ww1_l_ontemp_act` | Actual on temperature (scaled) |
| `pellematic_ww1_l_offtemp_act` | Actual off temperature (scaled) |
| `pellematic_ww1_l_pump` | Pump status (0=off, 1=on) |
| `pellematic_ww1_l_state` | DHW state code |
| `pellematic_ww1_statetext{component="..."}` | State text components (value=1.0) |

#### Pellet Boiler (`pellematic_pe1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_pe1_l_temp_act` | Actual boiler temperature (scaled) |
| `pellematic_pe1_l_temp_set` | Set boiler temperature (scaled) |
| `pellematic_pe1_l_modulation` | Boiler modulation level |
| `pellematic_pe1_l_lowpressure` | Current low pressure |
| `pellematic_pe1_l_lowpressure_set` | Set low pressure |
| `pellematic_pe1_l_fluegas` | Flue gas temperature |
| `pellematic_pe1_l_uw_speed` | Exhaust fan speed |
| `pellematic_pe1_l_uw` | Exhaust fan value |
| `pellematic_pe1_l_uw_release` | Exhaust fan release value |
| `pellematic_pe1_l_starts` | Total burner starts |
| `pellematic_pe1_l_runtime` | Total runtime (scaled) |
| `pellematic_pe1_l_storage_fill` | Storage tank fill level |
| `pellematic_pe1_statetext{component="..."}` | State text components (value=1.0) |

Additional pe1 fields (`L_frt_temp_act`, `L_frt_temp_set`, `L_frt_temp_end`, `L_br`, `L_ak`, `L_not`, `L_stb`, `L_type`, `L_storage_min`, `L_storage_max`, `L_storage_popper`, `mode`) are also exposed.

## Metric Scaling

The exporter applies automatic scaling based on field names. The first matching rule is applied:

| Field Contains | Scaling | Example |
|----------------|---------|---------|
| `temp` | Divided by 10 | Raw `185` becomes `18.5` |
| `runtime` | Multiplied by 3600 | Raw `775` becomes `2,790,000` (seconds) |
| `humidity` or `hum` | Divided by 10 | Raw `404` becomes `40.4` |
| `starts`, `modulation`, `lowpressure`, `_uw`, `_fluegas`, `storage_fill`, `pellets` | No scaling | Raw value used as-is |
| All other fields | No scaling | Raw value used as-is |

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
staticcheck ./...       # Static analysis
```

### Cross-Compilation

```bash
GOOS=linux GOARCH=amd64 go build -o oekofen-pellematic-exporter-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o oekofen-pellematic-exporter-linux-arm64 .
```

## Alerting Recommendations

Consider alerting on:
- `pellematic_scrape_errors_total` increasing (boiler unreachable)
- `pellematic_system_l_errors > 0` (active boiler errors)
- `pellematic_wireless1_l_wireless_batt < 20` (low battery on wireless sensor)
- Temperature deviations beyond expected ranges

## Troubleshooting

### Connection Errors

If you see "Failed to fetch data" errors:
- Verify the `-url` parameter points to the correct Pellematic JSON endpoint
- Check network connectivity to the boiler
- Ensure the boiler's web interface is accessible (try opening the URL in a browser)

### No Metrics Exposed

If no section metrics appear:
- Check the exporter logs for errors (use `-log=development` for verbose output)
- The boiler may be offline; only the scrape error/success metrics will be present until connectivity is restored
- Ensure the boiler returns valid JSON at the configured URL

### Incorrect Values

- Temperatures are automatically divided by 10 (the boiler reports them as integers multiplied by 10)
- Runtime values are multiplied by 3600 (converted from hours to seconds)
- If values look wrong for a specific field, check the scaling logic in `processValue()` in `collector.go`

## License

MIT
