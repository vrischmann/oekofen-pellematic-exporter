# Agent Guidelines for oekofen-pellematic-exporter

## Project Overview

A Go-based Prometheus exporter for Oekofen Pellematic pellet heating systems. The exporter fetches JSON data from the boiler's web interface and converts it to Prometheus metrics for monitoring and alerting.

The module and Dockerfile pin Go 1.25.7 (`go.mod` requires `go 1.25.7`, `Dockerfile` uses `golang:1.25.7`).

## Code Validation Commands

Before committing changes, always run the following commands to ensure code quality:

```bash
# Compile the project
go build ./...

# Check code formatting
gofmt -d -e .

# Reformat code if needed (fix formatting issues)
gofmt -s -w .

# Run static analysis (use: just lint)
staticcheck ./...

# Run tests
go test -v -timeout=60s ./...
```

**Note**: Always use these exact arguments when running these tools, as they match the project's conventions.

## Running the Project

To run the exporter locally:

```bash
go run .
```

With custom configuration:

```bash
go run . -url http://192.168.1.100/pellematic_full.json -addr :48400
```

### Manual Testing

Use the provided test data file for testing the collector:

```bash
# Serve the test data
python3 -m http.server 8000

# Run against the test data
go run . -url http://localhost:8000/testdata/pellematic.json
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
├── main.go              # Application entry point, HTTP server setup, graceful shutdown
├── collector.go         # Metrics collection, processing, and scraping logic
├── collector_test.go    # Tests for collector, metric helpers, and statetext processing
├── config.go            # CLI flag/env var parsing and logger setup
├── metrics.go           # Metric naming and label building utilities
├── Dockerfile           # Multi-arch Docker build (distroless, nonroot)
├── justfile             # Build recipes (docker/podman images, lint)
├── testdata/
│   └── pellematic.json  # Example JSON data from boiler (full format)
├── .github/
│   └── workflows/
│       └── container.yml  # CI: builds and pushes the multi-arch image to GHCR
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
└── .gitignore           # Git ignore patterns (excludes binaries)
```

The `.pi/` directory (Pi agent state) may exist locally but is gitignored and is not part of the repository.

## Key Components

### Main (`main.go`)

The entry point:
- Parses configuration and sets up logging
- Creates a Prometheus registry and registers the collector
- Starts the collector in a background goroutine (driven by context)
- Serves metrics via an HTTP handler on `/metrics`
- Handles graceful shutdown on `SIGINT` (5-second timeout)

### Collector (`collector.go`)

The core component, responsible for:
- Fetching JSON data from the Pellematic boiler endpoint
- Decoding ISO-8859-1 response body
- Processing and scaling metric values
- Managing connection state (online/offline)
- Exposing Prometheus metrics via `Describe`/`Collect` interface
- Running a periodic refresh loop via `Start(ctx)` (30s interval)

### Config (`config.go`)

Configuration via CLI flags with environment variable fallbacks:
- `-url` / `BOILER_URL`: Pellematic full JSON endpoint URL (default: `http://localhost/pellematic_full.json`)
- `-addr` / `LISTEN_ADDR`: HTTP server listen address (default: `:48400`)
- `-log` / `LOG_MODE`: Logging mode, `development` or `production` (default: `development`)

Environment variables serve as defaults; CLI flags take precedence.

### Metric Naming (`metrics.go`)

Metric names follow the pattern: `pellematic_{section}_{field}`

The `cleanLabelName` function lowercases field names and strips the `L_` prefix. The `componentName` parameter in `buildMetricName` is currently always passed as `""` and unused.

### Tests (`collector_test.go`)

Table-driven tests covering:
- Full integration: fetch, parse, and metric collection against `testdata/pellematic.json`
- Factor scaling verification (temperatures, raw values)
- Sentinel value filtering
- String field and `*_info` key skipping
- Statetext component splitting
- Error scenarios (server errors, invalid JSON, connection refused)
- Unit tests for helpers: `isSentinelValue`, `toFloat64`, `cleanLabelName`, `buildMetricName`, `processValue`, `processStateText`

Uses `testify/require` for assertions and `zaptest` for test logging. Test helpers create `httptest.Server` instances to mock the boiler HTTP endpoint.

## Data Processing Rules

### Full JSON Format (primary)

When using the full JSON endpoint (`pellematic_full.json`), each field is a metadata object:
```json
"L_temp_act": {"val": 523, "unit": "°C", "factor": 0.1, "min": -32768, "max": 32767, "text": "PE T Chaudière"}
```

Scaling is **data-driven**: the `factor` field is applied as `val * factor`. Metric help text uses the `text` field. String-valued fields and `*_info` section metadata keys are skipped.

### Legacy Scaling (non-full format fallback)

`processValue()` checks field names **in order**, returning on the first match. Used only when values are plain scalars (non-full JSON format):

| Check Order | Field Pattern | Operation | Notes |
|-------------|---------------|-----------|-------|
| 1 | Contains `temp` | Divide by 10.0 | Matches any field with "temp" in the name |
| 2 | Contains `runtime` | Multiply by 3600.0 | Catches `avg_runtime`, `runtimeburner`, `resttimeburner` too |
| 3 | Contains `avg_runtime` | Multiply by 60.0 | **Unreachable**: shadowed by check 2 |
| 4 | Contains `runtimeburner` or `resttimeburner` | No scaling | **Unreachable**: shadowed by check 2 |
| 5 | Contains `starts` | No scaling | |
| 6 | Contains `humidity` or `hum` | Divide by 10.0 | |
| 7 | Contains `_uw`, `_fluegas`, `modulation`, `lowpressure` | No scaling | |
| 8 | Contains `storage_fill` or `pellets` | No scaling | |
| 9 | (default) | No scaling (raw value) | All other fields |

**Note**: These legacy heuristics have known shadowing bugs (checks 3, 4). The full format avoids these entirely by using the `factor` field.

### Sentinel Values

These integer values indicate unavailable data and are skipped:
- `32765`
- `32767`
- `-32768`

### Section Handling

- **forecast section**: Completely skipped (not processed)
- **`*_info` keys** (e.g., `system_info`, `pe_info`): Skipped (section metadata, full format only)
- **String-valued fields**: Skipped (e.g., `L_source`, `L_location`, `name`)
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

The boiler's JSON is not always valid. The code applies a string replacement (`L_statetext:` to `L_statetext":`) to the decoded body string before unmarshalling. This fixes cases where the statetext value is missing its closing quote.

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
- Use `newTestCollector()` helper to create a collector backed by `httptest.Server`
- Use `gatherMetrics()` to collect and inspect metric values
- Use `requireMetric()` and `requireNoMetric()` for assertions
- Test both success and failure scenarios
- Verify metric names, values, and labels

## Common Tasks

### Adding a New Metric

1. Identify the JSON field in `testdata/pellematic.json`
2. The collector automatically processes numeric fields from the full format (using `val` and `factor`)
3. For the legacy non-full format, update `processValue()` in `collector.go` (add the check **before** any broader pattern)
4. Update the README.md to document the new metric

### Adding a New Data Section

1. The collector automatically handles new sections in `updateMetrics()`
2. If special processing is needed, update `processSection()` or `updateMetrics()`
3. Document the section in the README.md

### Fixing JSON Parsing Issues

If the boiler's JSON format changes:
1. Capture the actual JSON output
2. Update the test data file (`testdata/pellematic.json`)
3. Add string replacements in `fetchData()` (applied to the decoded body string before unmarshalling)
4. Test with real data

## Dependencies

### Core Dependencies

- `github.com/prometheus/client_golang` v1.23.2 - Prometheus client library
- `go.uber.org/zap` v1.27.1 - Structured logging
- `golang.org/x/text` v0.32.0 - Character encoding support (ISO-8859-1)

### Test Dependencies

- `github.com/stretchr/testify` v1.11.1 - Test assertions

## Build Artifacts

The following binaries are ignored by Git:
- `exporter`
- `oekofen-pellematic-exporter`

### Container CI

GitHub Actions (`.github/workflows/container.yml`) builds the multi-arch image and pushes it to `ghcr.io/vrischmann/oekofen-pellematic-exporter`. It triggers on pushes to `master`, on `v*` tags, and on manual dispatch. No extra credentials are required (it uses the built-in `GITHUB_TOKEN`).

When building for distribution:
```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o oekofen-pellematic-exporter-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o oekofen-pellematic-exporter-linux-arm64 .
```

## Monitoring Best Practices

### Prometheus Scrape Interval

Match the exporter's refresh interval (hardcoded 30s):
- Set `scrape_interval: 30s` in Prometheus
- Avoid scraping more frequently than data refreshes

### Alerting Recommendations

Consider alerting on:
- `pellematic_scrape_errors_total` increasing
- `pellematic_system_errors` > 0
- `pellematic_wireless1_wireless_batt` < 20 (low battery)
- Temperature deviations beyond normal ranges
