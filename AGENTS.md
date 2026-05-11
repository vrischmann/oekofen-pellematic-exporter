# Agent Guidelines for oekofen-pellematic-exporter

## Project Overview

A Go-based Prometheus exporter for Oekofen Pellematic pellet heating systems. The exporter fetches JSON data from the boiler's web interface and converts it to Prometheus metrics for monitoring and alerting.

## Code Validation Commands

Before committing changes, always run the following commands to ensure code quality:

```bash
# Compile the project
go build ./...

# Check code formatting
gofmt -d -e

# Reformat code if needed (fix formatting issues)
gofmt -s -w .

# Run static analysis
staticcheck ./...

# Run tests
go test -v -timeout=60s
```

**Note**: Always use these exact arguments when running these tools, as they match the project's conventions.

## Running the Project

To run the exporter locally:

```bash
go run oekofen-pellematic-exporter
```

With custom configuration:

```bash
go run oekofen-pellematic-exporter -url http://192.168.1.100/pellematic.json -addr :8080 -interval 30s
```

### Manual Testing

Use the provided `pellematic.json` file for testing the collector:

```bash
# Serve the example file
python3 -m http.server 8000

# Run the exporter against the local server
go run . -url http://localhost:8000/pellematic.json
```

## Dependency Management

### Adding Dependencies

```bash
go get <library>@latest
```

**Important**: NEVER use `go get -u` to update dependencies. Always specify the version explicitly or use `@latest`.

### Cleaning Up Dependencies

```bash
go mod tidy
```

Run `go mod tidy` after:
- Adding new dependencies
- Removing unused dependencies
- Modifying imports

## Project Structure

```
.
├── main.go           # Application entry point, HTTP server setup, graceful shutdown
├── collector.go      # Metrics collection, processing, and scraping logic
├── config.go         # CLI flag parsing and logger setup
├── metrics.go        # Metric naming and label building utilities
├── pellematic.json   # Example JSON data from boiler
├── go.mod            # Go module definition
├── go.sum            # Dependency checksums
└── .gitignore        # Git ignore patterns (excludes binaries)
```

## Key Components

### Main (`main.go`)

The entry point:
- Parses configuration and sets up logging
- Creates a Prometheus registry and registers the collector
- Starts the collector in a background goroutine (driven by context)
- Serves metrics via an HTTP handler
- Handles graceful shutdown on `SIGINT` (5-second timeout)

### Collector (`collector.go`)

The core component, responsible for:
- Fetching JSON data from the Pellematic boiler endpoint
- Decoding ISO-8859-1 response body
- Processing and scaling metric values
- Managing connection state (online/offline)
- Exposing Prometheus metrics via `Describe`/`Collect` interface
- Running a periodic refresh loop via `Start(ctx)`

### Config (`config.go`)

Configuration via command-line flags:
- `-url`: Pellematic JSON endpoint URL (default: `http://localhost/pellematic.json`)
- `-addr`: HTTP server listen address (default: `:8080`)
- `-path`: Metrics endpoint path (default: `/metrics`)
- `-interval`: Data refresh interval (default: `30s`)
- `-log`: Logging mode, `development` or `production` (default: `development`)

### Metric Naming (`metrics.go`)

Metric names follow the pattern: `pellematic_{section}_{field}`

The `cleanLabelName` function lowercases field names and strips the `L_` prefix. The `componentName` parameter in `buildMetricName` is currently always passed as `""` and unused.

## Data Processing Rules

### Scaling Rules

`processValue()` checks field names **in order**, returning on the first match. This means earlier, broader patterns shadow later, more specific ones:

| Check Order | Field Pattern | Operation | Notes |
|-------------|---------------|-----------|-------|
| 1 | Contains `temp` | Divide by 10.0 | Matches any field with "temp" in the name |
| 2 | Contains `runtime` | Multiply by 3600.0 | **Catches `avg_runtime`, `runtimeburner`, `resttimeburner` too** |
| 3 | Contains `avg_runtime` | Multiply by 60.0 | **Unreachable**: shadowed by check 2 |
| 4 | Contains `runtimeburner` or `resttimeburner` | No scaling | **Unreachable**: shadowed by check 2 |
| 5 | Contains `starts` | No scaling | |
| 6 | Contains `humidity` or `hum` | Divide by 10.0 | |
| 7 | Contains `_uw`, `_fluegas`, `modulation`, `lowpressure` | No scaling | |
| 8 | Contains `storage_fill` or `pellets` | No scaling | |
| 9 | (default) | No scaling (raw value) | All other fields |

**Known issue**: Checks 3 and 4 are dead code because `avg_runtime` contains `runtime`, and `runtimeburner`/`resttimeburner` also contain `runtime`. They all match check 2 first. If you fix this, reorder the checks so more specific patterns come before broader ones (e.g., check `avg_runtime` before `runtime`).

### Sentinel Values

These integer values indicate unavailable data and are skipped:
- `32765`
- `32767`
- `-32768`

### Section Handling

- **forecast section**: Completely skipped (not processed)
- **All other sections**: Processed as nested maps if the value is `map[string]interface{}`

### State Text Processing

State text (`L_statetext` fields) is split by `|` and exported as labeled metrics:
- Metric name: `pellematic_{section}_statetext`
- Label: `component`
- Value: Always `1.0` (presence indicator)

## Special Considerations

### Character Encoding

The Pellematic boiler returns data in **ISO-8859-1** encoding. The exporter decodes the response using `charmap.ISO8859_1.NewDecoder().Reader()`.

### JSON Quirks

The boiler's JSON is not always valid. The code attempts to apply a string replacement (`L_statetext:` to `L_statetext":`) but applies it to `bodyStr` (a Go string) while `json.Unmarshal` receives the original `body` byte slice. This means the fix is currently a no-op. If you need to fix malformed JSON, apply the replacement to the byte slice before unmarshalling.

### Error Handling

- Scrape errors increment the `pellematic_scrape_errors_total` counter
- Failed scrapes mark the collector as offline (application metrics stop being exposed, but the scrape error/success gauges remain)
- The `pellematic_scrape_last_success_timestamp_seconds` gauge only updates on successful scrapes

### Connection State

The collector tracks online/offline state with mutex protection:
- On successful fetch: metrics are updated and collector goes online
- On failed fetch: error counter increments, collector goes offline, existing metrics are cleared
- When offline: `Describe`/`Collect` only emit the scrape error/success metrics

## Testing Guidelines

### Unit Testing

When adding tests:
- Use table-driven tests for multiple test cases
- Mock HTTP responses for testing collector logic
- Test both success and failure scenarios
- Verify metric names, values, and labels

## Conventional Commits

Follow the conventional commit format:

```
<type>(<scope>): <description>
```

**Types:** `feat`, `fix`, `chore`, `refactor`, `docs`, `style`, `test`, `perf`

**Examples:**
- `feat(collector): add support for additional sensor types`
- `fix(collector): correct temperature scaling for negative values`
- `chore(deps): update prometheus client to v1.23.2`
- `docs(readme): add docker compose example`

## Common Tasks

### Adding a New Metric

1. Identify the JSON field in `pellematic.json`
2. The collector will automatically process most numeric fields
3. If special scaling is needed, update `processValue()` in `collector.go` (add the check **before** any broader pattern that would match it)
4. Update the README.md to document the new metric

### Adding a New Data Section

1. The collector automatically handles new sections in `updateMetrics()`
2. If special processing is needed, update `processSection()` or `updateMetrics()`
3. Document the section in the README.md

### Fixing JSON Parsing Issues

If the boiler's JSON format changes:
1. Capture the actual JSON output
2. Update the `pellematic.json` example file
3. Add string replacements in `fetchData()` (make sure to apply to the byte slice actually passed to `json.Unmarshal`)
4. Test with real data

## Dependencies

### Core Dependencies

- `github.com/prometheus/client_golang` v1.23.2 - Prometheus client library
- `go.uber.org/zap` v1.27.1 - Structured logging
- `golang.org/x/text` v0.32.0 - Character encoding support (ISO-8859-1)

## Build Artifacts

The following binaries are ignored by Git:
- `exporter`
- `oekofen-pellematic-exporter`

When building for distribution:
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o oekofen-pellematic-exporter-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o oekofen-pellematic-exporter-linux-arm64 .
```

## Monitoring Best Practices

### Prometheus Scrape Interval

Match the exporter's refresh interval:
- If `-interval=30s`, set `scrape_interval: 30s` in Prometheus
- Avoid scraping more frequently than data refreshes

### Alerting Recommendations

Consider alerting on:
- `pellematic_scrape_errors_total` increasing
- `pellematic_system_l_errors` > 0
- `pellematic_wireless1_l_wireless_batt` < 20 (low battery)
- Temperature deviations beyond normal ranges
