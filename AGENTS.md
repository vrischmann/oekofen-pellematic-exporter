# Agent Guidelines for oekofen-pellematic-exporter

## Project Overview

This is a Go-based Prometheus exporter for Ökofen Pellematic pellet heating systems. The exporter fetches JSON data from the boiler's web interface and converts it to Prometheus metrics for monitoring and alerting.

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
├── main.go           # Application entry point, HTTP server setup
├── collector.go      # Metrics collection and processing logic
├── config.go         # Configuration parsing and logging setup
├── metrics.go        # Metric naming and label building utilities
├── pellematic.json   # Example JSON data from boiler
├── go.mod            # Go module definition
├── go.sum            # Dependency checksums
└── .gitignore        # Git ignore patterns (excludes binaries)
```

## Key Components

### Collector (`collector.go`)

The collector is responsible for:
- Fetching JSON data from the Pellematic boiler endpoint
- Processing and scaling metric values
- Managing connection state (online/offline)
- Exposing Prometheus metrics
- Handling the refresh interval ticker

**Important**: The collector handles ISO-8859-1 encoding and applies fixes to malformed JSON (e.g., `L_statetext:` → `L_statetext":`).

### Config (`config.go`)

Configuration is handled via command-line flags using Go's `flag` package:
- `-url`: Pellematic JSON endpoint URL
- `-addr`: HTTP server listen address
- `-path`: Metrics endpoint path
- `-interval`: Data refresh interval
- `-log`: Logging mode (development/production)

### Metric Naming (`metrics.go`)

Metric names follow the pattern: `pellematic_{section}_{field}`

Label names are cleaned by:
- Lowercasing
- Removing the `L_` prefix

## Data Processing Rules

### Scaling Rules

When processing metric values, the following scaling is applied:

| Field Pattern | Operation |
|---------------|-----------|
| Contains `temp` | Divide by 10.0 |
| Contains `runtime` | Multiply by 3600.0 (to hours) |
| Contains `avg_runtime` | Multiply by 60.0 (to minutes) |
| Contains `humidity` or `hum` | Divide by 10.0 |
| Contains `_uw`, `_fluegas`, `modulation`, `lowpressure` | No scaling |
| Contains `storage_fill`, `pellets` | No scaling |
| Contains `starts` | No scaling |

### Sentinel Values

These integer values indicate unavailable data and are skipped:
- `32765`
- `32767`
- `-32768`

### Section Handling

- **forecast section**: Completely skipped (not processed)
- **All other sections**: Processed as nested maps

### State Text Processing

State text (e.g., `L_statetext`) is split by `|` and exported as labeled metrics:
- Metric name: `pellematic_{section}_statetext`
- Label: `component`
- Value: Always `1.0` (presence indicator)

## Special Considerations

### Character Encoding

The Pellematic boiler returns data in **ISO-8859-1** encoding. The exporter handles this by decoding the response using `charmap.ISO8859_1`.

### JSON Quirks

The boiler's JSON is not always valid. The exporter applies string replacements to fix common issues:
- `L_statetext:` → `L_statetext":`

### Error Handling

- Scrape errors increment the `pellematic_scrape_errors_total` counter
- Failed scrapes mark the collector as offline (metrics stop being exposed)
- The `pellematic_scrape_last_success_timestamp_seconds` gauge only updates on successful scrapes

## Testing Guidelines

### Manual Testing

Use the provided `pellematic.json` file for testing the collector:

```bash
# Serve the example file
python3 -m http.server 8000

# Run the exporter against the local server
go run . -url http://localhost:8000/pellematic.json
```

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

**Types:**
- `feat`: New functionality
- `fix`: Bug fixes
- `chore`: Maintenance, dependencies
- `refactor`: Code restructuring
- `docs`: Documentation changes
- `style`: Formatting, linting
- `test`: Tests
- `perf`: Performance improvements

**Examples:**
- `feat(collector): add support for additional sensor types`
- `fix(collector): correct temperature scaling for negative values`
- `chore(deps): update prometheus client to v1.23.2`
- `docs(readme): add docker compose example`

## Common Tasks

### Adding a New Metric

1. Identify the JSON field in `pellematic.json`
2. The collector will automatically process most fields
3. If special scaling is needed, update `processValue()` in `collector.go`
4. Update the README.md to document the new metric

### Adding a New Data Section

1. The collector automatically handles new sections
2. If special processing is needed, update `processSection()` or `updateMetrics()`
3. Document the section in the README.md

### Fixing JSON Parsing Issues

If the boiler's JSON format changes:
1. Capture the actual JSON output
2. Update the `pellematic.json` example
3. Add string replacements in `fetchData()` if needed
4. Test with real data

## Dependencies

### Core Dependencies

- `github.com/prometheus/client_golang` - Prometheus client library
- `go.uber.org/zap` - Structured logging

### Indirect Dependencies

- `github.com/beorn7/perks` - Quantile computation
- `github.com/cespare/xxhash/v2` - Fast hashing
- `golang.org/x/text/encoding/charmap` - Character encoding support

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

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o oekofen-pellematic-exporter-darwin-amd64 .
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

## Git Workflow

1. Create a feature branch from main
2. Make changes and run validation commands
3. Test thoroughly with real data when possible
4. Commit with conventional commit format
5. Push and create a pull request

## Getting Help

- Review the `pellematic.json` example for data structure
- Check the boiler's documentation for field meanings
- Enable development logging (`-log=development`) for debug output
- Use Prometheus UI to inspect exposed metrics
