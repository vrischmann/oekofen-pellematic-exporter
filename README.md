# Oekofen Pellematic Prometheus Exporter

A Prometheus exporter for Oekofen Pellematic pellet heating systems. It fetches JSON data from the boiler's built-in web interface and exposes it as Prometheus metrics for monitoring and alerting.

## Features

- Fetches metrics from Oekofen Pellematic boilers via their full JSON endpoint
- Data-driven metric scaling using the `factor` field from the boiler's telemetry
- Human-readable metric descriptions from the boiler's own `text` metadata
- Backward compatible with the non-full JSON endpoint
- Graceful shutdown on SIGINT
- Development and production logging modes (structured JSON in production)
- Online/offline tracking: when the boiler is unreachable, stale metrics are removed and errors are counted

## Supported Data Sections

The exporter processes all top-level sections from the Pellematic full JSON endpoint (except `forecast`, which is skipped). Typical sections include:

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
  -url http://192.168.1.100/pellematic_full.json \
  -addr :48400
```

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `http://localhost/pellematic_full.json` | Pellematic boiler full JSON endpoint URL |
| `-addr` | `:48400` | HTTP server listen address |
| `-log` | `development` | Log mode: `development` or `production` |

### Docker

```bash
docker run -d \
  --name pellematic-exporter \
  -p 48400:48400 \
  oekofen-pellematic-exporter \
  -url http://192.168.1.100/pellematic_full.json \
  -log production
```

### Docker Compose

```yaml
services:
  pellematic-exporter:
    image: oekofen-pellematic-exporter
    container_name: pellematic-exporter
    ports:
      - "48400:48400"
    command:
      - "-url=http://192.168.1.100/pellematic_full.json"
      - "-addr=:48400"
      - "-log=production"
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

## Full JSON Format

When using the full JSON endpoint (`pellematic_full.json`), each field value is an object containing rich metadata:

```json
"L_temp_act": {"val": 523, "unit": "°C", "factor": 0.1, "min": -32768, "max": 32767, "text": "PE T Chaudière"}
```

The exporter uses this metadata to:
- **Scale values** using the `factor` field (e.g., `523 * 0.1 = 52.3°C`)
- **Provide descriptions** via the `text` field (used as Prometheus metric help)
- **Skip unavailable data** when values match sentinel thresholds (`min`/`max` bounds)

String-valued fields (names, URLs, update timestamps) are automatically skipped.

### Backward Compatibility

The exporter also supports the non-full JSON endpoint (`pellematic.json`) where values are plain scalars. In this mode, heuristic-based scaling is applied (see legacy scaling rules below). The format is auto-detected per field.

## Available Metrics

### Scrape Metrics (always present)

| Metric | Type | Description |
|--------|------|-------------|
| `pellematic_scrape_errors_total` | counter | Total number of scrape errors from the Pellematic boiler |
| `pellematic_scrape_last_success_timestamp_seconds` | gauge | Unix timestamp of last successful scrape |

### Example Metrics by Section

Below are the most commonly used metrics. Metric help text comes directly from the boiler's `text` metadata when using the full JSON endpoint.

#### System (`pellematic_system_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_system_ambient` | T extérieure (ambient temperature) |
| `pellematic_system_errors` | Défaut (active errors) |
| `pellematic_system_usb_stick` | Usb connectée |
| `pellematic_system_existing_boiler` | T mes |
| `pellematic_system_mode` | Mode (0=Off, 1=Auto, 2=DHW) |

#### Weather (`pellematic_weather_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_weather_temp` | Temp. actuelle (outdoor temperature) |
| `pellematic_weather_clouds` | Couverture nuageuse actuelle |
| `pellematic_weather_forecast_temp` | Température moyenne |
| `pellematic_weather_forecast_clouds` | Nébulosité moyenne |
| `pellematic_weather_cloud_limit` | Seuil météo |
| `pellematic_weather_hysteresys` | Hyst. temp amb. pour arrêt fonction écolo |
| `pellematic_weather_offtemp` | T°C ext. de coupure |
| `pellematic_weather_lead` | Durée d'anticipation |
| `pellematic_weather_oekomode` | Mode écolo |

#### Heating Circuit 1 (`pellematic_hk1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_hk1_roomtemp_act` | T ambiante (actual room temperature) |
| `pellematic_hk1_roomtemp_set` | T Amb Cons (set room temperature) |
| `pellematic_hk1_flowtemp_act` | T Dep mes (actual flow temperature) |
| `pellematic_hk1_flowtemp_set` | T Dep cons (set flow temperature) |
| `pellematic_hk1_comfort` | T°C de confort |
| `pellematic_hk1_state` | Etat (state code) |
| `pellematic_hk1_pump` | Chf Pompe (0=Off, 1=On) |
| `pellematic_hk1_statetext{component="..."}` | State text components (value=1.0) |
| `pellematic_hk1_temp_heat` | T ambiance confort |
| `pellematic_hk1_temp_setback` | T ambiance réduit |
| `pellematic_hk1_oekomode` | Mode écolo |

#### Wireless Sensor (`pellematic_wireless1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_wireless1_wireless_temp` | Temperature |
| `pellematic_wireless1_wireless_hum` | Humidité de l'air |
| `pellematic_wireless1_wireless_rssi` | Signal (RSSI) |
| `pellematic_wireless1_wireless_batt` | Batterie (%) |

#### Domestic Hot Water (`pellematic_ww1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_ww1_temp_set` | Consigne ECS (set temperature) |
| `pellematic_ww1_ontemp_act` | T démarrage (on temperature) |
| `pellematic_ww1_offtemp_act` | T arrêt (off temperature) |
| `pellematic_ww1_pump` | Pompe (0=Off, 1=On) |
| `pellematic_ww1_state` | Etat (state code) |
| `pellematic_ww1_statetext{component="..."}` | State text components (value=1.0) |
| `pellematic_ww1_temp_max_set` | Consigne ECS (max set temperature) |
| `pellematic_ww1_smartstart` | Charge ECS anticipée |
| `pellematic_ww1_use_boiler_heat` | Energie restante utilisée |

#### Pellet Boiler (`pellematic_pe1_*`)

| Metric | Description |
|--------|-------------|
| `pellematic_pe1_temp_act` | PE T Chaudière (actual boiler temperature) |
| `pellematic_pe1_temp_set` | TC Ret cons (set temperature) |
| `pellematic_pe1_frt_temp_act` | T flamme (flame temperature) |
| `pellematic_pe1_modulation` | Niveau de Modulation (%) |
| `pellematic_pe1_runtime` | t fonct brûleur (total runtime, in hours) |
| `pellematic_pe1_avg_runtime` | PE t moyen brûleur (avg runtime, in minutes) |
| `pellematic_pe1_runtimeburner` | t marche vis brûleur (burner runtime) |
| `pellematic_pe1_resttimeburner` | temps pause (burner rest time) |
| `pellematic_pe1_starts` | démarrage brûl (total burner starts) |
| `pellematic_pe1_lowpressure` | Dépression |
| `pellematic_pe1_lowpressure_set` | Valeur cons. dépression |
| `pellematic_pe1_fluegas` | vitesse V fumées |
| `pellematic_pe1_uw_speed` | vitesse UW (%) |
| `pellematic_pe1_uw` | PE vitesse UW |
| `pellematic_pe1_uw_release` | T limite |
| `pellematic_pe1_storage_fill` | quantité granulés dans silo (kg) |
| `pellematic_pe1_storage_min` | Seuil alerte granulés (kg) |
| `pellematic_pe1_storage_max` | Capacité max. stockage (kg) |
| `pellematic_pe1_storage_popper` | Pellet dans trémie (kg) |
| `pellematic_pe1_statetext{component="..."}` | State text components (value=1.0) |

Additional pe1 fields (`L_br`, `L_ak`, `L_not`, `L_stb`, `L_type`, `L_currentairflow`, `mode`) are also exposed.

## Metric Scaling

### Full JSON Format (recommended)

When using the full JSON endpoint, scaling is **data-driven**: each field carries a `factor` that the exporter applies automatically. For example:

| Field | Raw `val` | `factor` | Exported Value | Unit |
|-------|-----------|----------|----------------|------|
| `L_temp_act` | 523 | 0.1 | 52.3 | °C |
| `L_runtime` | 1672 | 1 | 1672 | h |
| `L_runtimeburner` | 0 | 0.01 | 0 | zs |
| `L_lowpressure` | 449 | 0.1 | 44.9 | EH |
| `L_starts` | 1605 | 1 | 1605 | - |

This eliminates the heuristic-based scaling used with the non-full endpoint, and correctly handles fields that were previously mis-scaled (e.g., `L_lowpressure`, `L_runtimeburner`).

**Sentinel values** `32765`, `32767`, and `-32768` indicate unavailable data and are skipped entirely.

### Legacy Scaling (non-full format)

When using the non-full JSON endpoint, heuristic scaling is applied based on field name patterns:

| Field Contains | Scaling | Example |
|----------------|---------|---------|
| `temp` | Divided by 10 | Raw `185` becomes `18.5` |
| `runtime` | Multiplied by 3600 | Raw `775` becomes `2,790,000` (seconds) |
| `humidity` or `hum` | Divided by 10 | Raw `404` becomes `40.4` |
| `starts`, `modulation`, `lowpressure`, `_uw`, `_fluegas`, `storage_fill`, `pellets` | No scaling | Raw value used as-is |
| All other fields | No scaling | Raw value used as-is |

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
- `pellematic_system_errors > 0` (active boiler errors)
- `pellematic_wireless1_wireless_batt < 20` (low battery on wireless sensor)
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

- When using the full endpoint, values are scaled by the `factor` field provided by the boiler itself
- When using the non-full endpoint, temperatures are automatically divided by 10 and runtimes multiplied by 3600
- Sentinel values (32765, 32767, -32768) are automatically filtered out

## License

MIT
