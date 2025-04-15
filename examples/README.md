# OpenTelemetry Client Example for OpenObserve

This example demonstrates how to use the `otel-client` package to integrate OpenTelemetry with a simple HTTP server in Go. The server exposes a `/hello` endpoint, logs requests using `slog`, records metrics (accepted/failed requests and latency), and traces operations, all sent to an OpenObserve instance.

## Prerequisites

- **Go**: Version 1.21 or later.
- **OpenObserve**: Running locally on `http://localhost:5081` with OTLP endpoint at `localhost:5081` (or your custom endpoint).
- **curl**: For testing the HTTP endpoint.
- **Docker**: If running OpenObserve in a container.

## Project Structure

```
.
├── examples
│   └── example.go
├── go.mod
├── go.sum
├── otel
│   ├── messaging.go
│   ├── otel.go
│   ├── slog.go
│   └── utils.go
└── README.md
```

- `examples/example.go`: The example HTTP server.
- `otel/`: The OpenTelemetry client package.
- `go.mod`: Module dependencies.

## Setup

1. **Navigate to the Project Directory**:

   If you’ve cloned a repository:

   ```bash
   git clone <repository-url>
   cd otel-client
   ```

   Or, if working locally:

   ```bash
   cd ~/Documents/otel-client
   ```

2. **Verify OpenObserve**:

   Ensure OpenObserve is running:

   ```bash
   docker ps
   ```

   Confirm it’s accessible at `http://localhost:5081`. If using a custom setup, verify the OTLP endpoint (default: `localhost:4317`, but configured as `localhost:5081` in this example).

3. **Update OpenObserve Configuration**:

   Open `examples/example.go` and verify the `otel.Config`:

   ```go
   config := otel.Config{
       Host:         "localhost:5081",
       Token:        "cm9vdEBleGFtcGxlLmNvbTpDb21wbGV4cGFzcyMxMjM=",
       ServiceName:  "example-service",
       Environment:  "development",
       Organization: "example-org",
       StreamName:   "example-stream",
       SampleRate:   1.0,
   }
   ```

   - `Host`: Set to `localhost:5081` for local OpenObserve OTLP ingestion.
   - `Token`: Base64-encoded `root@example.com:Complexpass#123`.
   - `Organization` and `StreamName`: Match your OpenObserve setup.
   - `SampleRate`: 1.0 to capture all traces.

   Update these if using a different endpoint or credentials.

4. **Install Dependencies**:

   ```bash
   go mod tidy
   ```

## Running the Example

Start the server:

```bash
go run examples/example.go
```

The server runs on `http://localhost:8080` and logs:

```
{"time":"2025-04-15T12:00:00Z","level":"INFO","msg":"Test log from main","app":"example-service"}
{"time":"2025-04-15T12:00:02Z","level":"INFO","msg":"Starting HTTP server on :8080"}
```

It will:
- Serve `/hello` requests.
- Log requests and outcomes with trace context.
- Record metrics (accepted/failed requests, latency).
- Send traces, metrics, and logs to OpenObserve.

## Testing with curl

Test the `/hello` endpoint, which has a ~1% failure rate to simulate errors.

1. **Successful Request**:

   ```bash
   curl http://localhost:8080/hello
   ```

   **Output**:

   ```
   Hello, World!
   ```

   **Logs**:

   ```
   {"time":"2025-04-15T12:01:00Z","level":"INFO","msg":"Processing request","method":"GET","path":"/hello","trace_id":"abc123","span_id":"def456"}
   {"time":"2025-04-15T12:01:00Z","level":"INFO","msg":"Request completed successfully","trace_id":"abc123","span_id":"def456"}
   ```

2. **Failed Request** (rare, ~1% chance):

   ```bash
   curl http://localhost:8080/hello
   ```

   **Output** (on failure):

   ```
   Internal Server Error
   ```

   **Logs**:

   ```
   {"time":"2025-04-15T12:02:00Z","level":"INFO","msg":"Processing request","method":"GET","path":"/hello","trace_id":"ghi789","span_id":"jkl012"}
   {"time":"2025-04-15T12:02:00Z","level":"ERROR","msg":"Request failed","error":"simulated request failure","trace_id":"ghi789","span_id":"jkl012"}
   ```

3. **Multiple Requests**:

   ```bash
   for i in {1..10}; do curl -s http://localhost:8080/hello; echo; done
   ```

   Generates metrics and logs for analysis in OpenObserve.

## Verifying in OpenObserve

Access OpenObserve at `http://localhost:5081`:

- **Logs**:
   - Go to Logs > Streams > `example-org` > `example-stream`.
   - Query `match_all('*')` or `match_all('Test log')` for “Last 5 minutes”.
   - Expect: `Test log from main`, `Processing request`, `Request completed successfully`, occasional `Request failed`.
   - Verify fields: `_timestamp`, `message`, `severity`, `trace_id`, `span_id`, `app`.
- **Traces**:
   - Check for `handleHello` spans under `example-service`.
   - Look for `OK` or error statuses with `trace_id` correlating to logs.
- **Metrics**:
   - `example_service_module_requests_accepted_total`: Successful requests.
   - `example_service_module_requests_failed_total`: Failed requests.
   - `example_service_module_request_duration_seconds`: Latency histogram.

## Stopping the Server

Press `Ctrl+C`. The server shuts down gracefully:

```
{"time":"2025-04-15T12:05:00Z","level":"INFO","msg":"Shutting down server..."}
{"time":"2025-04-15T12:05:00Z","level":"INFO","msg":"Server stopped"}
```

## Troubleshooting

- **No Logs in OpenObserve**:
   - Verify `localhost:5081` accepts OTLP (try `nc -zv localhost 5081` or test with `4317`).
   - Check OpenObserve logs:
     ```bash
     docker logs <openobserve-container-id>
     ```
   - Send a direct log:
     ```bash
     curl -u "root@example.com:Complexpass#123" -X POST \
     http://localhost:5081/api/example-org/example-stream/_json \
     -d '{"message":"direct test","app":"example-service"}'
     ```
   - Enable OTLP debug:
     ```bash
     export OTEL_LOG_LEVEL=debug
     go run examples/example.go
     ```
- **Delayed Logs**:
   - Check `_timestamp` in OpenObserve matches current time.
   - Restart OpenObserve:
     ```bash
     docker restart <openobserve-container-id>
     ```
- **Connection Errors**:
   - Confirm `Token`, `Organization`, and `StreamName`.
   - Test OTLP endpoint:
     ```bash
     curl http://localhost:5081
     ```
- **Dependency Issues**:
   - Run `go mod tidy`.
   - Use Go 1.21+.

## Notes

- Structured logging uses `slog` with a custom OpenTelemetry handler (`otel/slog.go`).
- Failure rate is ~1% for demonstration; adjust in `example.go` (`time.Now().UnixNano()%99`).
- Metrics include `endpoint="/hello"` for filtering.
- If logs don’t appear, verify OpenObserve’s OTLP port (e.g., `4317`) and update `Host` in `example.go`.

---
