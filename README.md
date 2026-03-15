# Ökofen Pellematic Prometheus Exporter

A Prometheus exporter for Ökofen Pellematic pellet heating systems. This exporter fetches data from the boiler's JSON endpoint and exposes it as Prometheus metrics for monitoring and alerting.

## Features

- Fetches metrics from Ökofen Pellematic boilers via their JSON endpoint
- Exposes Prometheus-formatted metrics on a configurable HTTP endpoint
- Automatic metric scaling (temperatures, runtimes, humidity, etc.)
- Graceful shutdown handling
- Development and production logging modes
- Connection status tracking with online/offline indicators

## Supported Data Sections

The exporter processes the following sections from the Pellematic JSON endpoint:

- **system**: System-wide metrics (ambient temperature, errors, USB stick status, mode)
- **weather**: Weather data (temperature, clouds, forecast, location)
- **hk1**: Heating circuit 1 (room temperature, flow temperature, pump status, state)
- **wireless1**: Wireless room sensor data (temperature, humidity, battery, RSSI)
- **ww1**: Domestic hot water (temperature, pump status, heating mode)
- **pe1**: Pellet boiler metrics (modulation, runtime, burner status, flue gas temperature)
- **error**: Error information

## Installation

### From Source

```bash
git clone https://github.com/yourusername/oekofen-pellematic-exporter.git
cd oekofen-pellematic-exporter
go build -o oekofen-pellematic-exporter .
```

### Binary

Download the pre-built binary for your platform from the [releases](https://github.com/yourusername/oekofen-pellematic-exporter/releases) page.

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

### Docker Usage

```bash
docker run -d \
  --name pellematic-exporter \
  -p 8080:8080 \
  -e PELLEMATIC_URL=http://192.168.1.100/pellematic.json \
  yourusername/oekofen-pellematic-exporter
```

### Docker Compose

```yaml
version: '3'
services:
  pellematic-exporter:
    image: yourusername/oekofen-pellematic-exporter
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

## Available Metrics

### General Metrics

- `pellematic_scrape_errors_total` - Total number of scrape errors
- `pellematic_scrape_last_success_timestamp_seconds` - Unix timestamp of last successful scrape

### System Metrics (`pellematic_system_*`)

- `pellematic_system_l_ambient` - Ambient temperature (°C)
- `pellematic_system_l_errors` - Number of active errors
- `pellematic_system_l_usb_stick` - USB stick status
- `pellematic_system_l_existing_boiler` - Existing boiler status
- `pellematic_system_mode` - System mode

### Weather Metrics (`pellematic_weather_*`)

- `pellematic_weather_l_temp` - Outdoor temperature (°C)
- `pellematic_weather_l_clouds` - Cloud coverage (%)
- `pellematic_weather_l_forecast_temp` - Forecast temperature (°C)
- `pellematic_weather_l_forecast_clouds` - Forecast cloud coverage (%)
- `pellematic_weather_l_starttime` - Start time (decimal hours)
- `pellematic_weather_l_endtime` - End time (decimal hours)

### Heating Circuit 1 Metrics (`pellematic_hk1_*`)

- `pellematic_hk1_l_roomtemp_act` - Actual room temperature (°C)
- `pellematic_hk1_l_roomtemp_set` - Set room temperature (°C)
- `pellematic_hk1_l_flowtemp_act` - Actual flow temperature (°C)
- `pellematic_hk1_l_flowtemp_set` - Set flow temperature (°C)
- `pellematic_hk1_l_comfort` - Comfort mode status
- `pellematic_hk1_l_state` - Heating circuit state
- `pellematic_hk1_l_pump` - Pump status (0=off, 1=on)
- `pellematic_hk1_statetext{component="..."}` - State text components (labels)

### Wireless Sensor Metrics (`pellematic_wireless1_*`)

- `pellematic_wireless1_l_wireless_temp` - Wireless sensor temperature (°C)
- `pellematic_wireless1_l_wireless_hum` - Wireless sensor humidity (%)
- `pellematic_wireless1_l_wireless_rssi` - Signal strength (dBm)
- `pellematic_wireless1_l_wireless_batt` - Battery level (%)

### Domestic Hot Water Metrics (`pellematic_ww1_*`)

- `pellematic_ww1_l_temp_set` - Set temperature (°C)
- `pellematic_ww1_l_ontemp_act` - Actual on temperature (°C)
- `pellematic_ww1_l_offtemp_act` - Actual off temperature (°C)
- `pellematic_ww1_l_pump` - Pump status (0=off, 1=on)
- `pellematic_ww1_l_state` - DHW state
- `pellematic_ww1_statetext{component="..."}` - State text components (labels)

### Pellet Boiler Metrics (`pellematic_pe1_*`)

- `pellematic_pe1_l_temp_act` - Actual boiler temperature (°C)
- `pellematic_pe1_l_temp_set` - Set boiler temperature (°C)
- `pellematic_pe1_l_modulation` - Boiler modulation level
- `pellematic_pe1_l_runtimeburner` - Current burner runtime
- `pellematic_pe1_l_resttimeburner` - Burner rest time
- `pellematic_pe1_l_lowpressure` - Current low pressure
- `pellematic_pe1_l_lowpressure_set` - Set low pressure
- `pellematic_pe1_l_fluegas` - Flue gas temperature
- `pellematic_pe1_l_uw_speed` - Exhaust fan speed
- `pellematic_pe1_l_starts` - Total burner starts
- `pellematic_pe1_l_runtime` - Total runtime (hours)
- `pellematic_pe1_l_avg_runtime` - Average runtime (minutes)
- `pellematic_pe1_l_storage_fill` - Storage tank fill level
- `pellematic_pe1_statetext{component="..."}` - State text components (labels)

## Metric Scaling

The exporter applies the following scaling to raw values:

- **Temperatures** (fields containing `temp`): Divided by 10
- **Runtime** (fields containing `runtime`): Multiplied by 3600 (converted to hours)
- **Average Runtime** (fields containing `avg_runtime`): Multiplied by 60 (converted to minutes)
- **Humidity** (fields containing `humidity` or `hum`): Divided by 10
- **Other values**: No scaling applied

Sentinel values (32765, 32767, -32768) are skipped as they indicate unavailable data.

## Development

### Prerequisites

- Go 1.25.7 or later

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
# Format code
gofmt -s -w .

# Run static analysis
staticcheck ./...

# Build check
go build ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License.

## Troubleshooting

### Connection Errors

If you see "Failed to fetch data" errors:
- Verify the `-url` parameter points to the correct Pellematic JSON endpoint
- Check network connectivity to the boiler
- Ensure the boiler's web interface is accessible

### No Metrics Exposed

If metrics are not being exposed:
- Check the Prometheus exporter is running
- Verify the `-path` and `-addr` parameters
- Ensure the boiler is returning valid JSON data
- Check logs for any parsing errors

### Incorrect Temperature Values

If temperatures appear incorrect:
- The exporter expects temperatures to be stored as integers (multiply by 10)
- If your boiler uses a different format, you may need to modify the `processValue` function

## Acknowledgments

- [Prometheus](https://prometheus.io/) - Monitoring system and time series database
- [Ökofen](https://www.oekofen.com/) - Pellet heating systems
