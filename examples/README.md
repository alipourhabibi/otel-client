# OpenTelemetry Client Example for OpenObserver

This example demonstrates how to use the `otel-client` package to integrate OpenTelemetry with a simple HTTP server in Go. The server exposes a `/hello` endpoint, logs requests using `slog`, records metrics (accepted/failed requests and latency), and traces operations, all sent to an OpenObserver instance.

## Prerequisites

- **Go**: Version 1.21 or later.
- **OpenObserver**: Access to an OpenObserver instance with a valid endpoint and token.
- **curl**: For testing the HTTP endpoint.

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

1. **Clone the Repository** (if applicable):

   If this is part of a repository, clone it:

   ```bash
   git clone <repository-url>
   cd otel-client
   ```

If you're working locally, navigate to the project directory:

   ```bash
   cd ~/Documents/otel-client
   ```

2. **Update OpenObserver Configuration**:

   Open `examples/example.go` and update the `otel.Config` with your OpenObserver details:

   ```go
   config := otel.Config{
       Host:         "your-openobserver-endpoint:4317", // e.g., "api.openobserver.com:4317"
       Token:        "your-openobserver-token",
       ServiceName:  "example-service",
       Environment:  "development",
       Organization: "your-organization",
       StreamName:   "example-stream",
       SampleRate:   1.0, // Sample all traces
   }
   ```

   Replace:
    - `your-openobserver-endpoint:4317` with the actual OpenObserver gRPC endpoint.
    - `your-openobserver-token` with your authentication token.
    - `your-organization` with your OpenObserver organization ID.
    - `example-stream` with your desired stream name.

3. **Install Dependencies**:

   Ensure all dependencies are resolved:

   ```bash
   go mod tidy
   ```

## Running the Example

Run the example server:

```bash
go run examples/example.go
```

The server starts on `http://localhost:8080` and logs:

```
{"time":"2025-04-15T12:00:00Z","level":"INFO","msg":"Starting HTTP server on :8080"}
```

The server will:
- Handle requests at `/hello`.
- Log requests and responses with trace context.
- Record metrics (accepted/failed requests, latency).
- Send traces, metrics, and logs to OpenObserver.

## Testing with curl

Use `curl` to test the `/hello` endpoint. The server simulates a 10% failure rate to demonstrate error handling.

1. **Successful Request**:

   ```bash
   curl http://localhost:8080/hello
   ```

   **Expected Output**:

   ```
   Hello, World!
   ```

   **Server Logs** (example):

   ```
   {"time":"2025-04-15T12:01:00Z","level":"INFO","msg":"Processing request","method":"GET","path":"/hello","trace_id":"abc123","span_id":"def456"}
   {"time":"2025-04-15T12:01:00Z","level":"INFO","msg":"Request completed successfully","trace_id":"abc123","span_id":"def456"}
   ```

   This indicates a successful request, with metrics recorded for latency and accepted requests, and traces sent to OpenObserver.

2. **Failed Request** (may require multiple attempts due to 10% failure rate):

   ```bash
   curl http://localhost:8080/hello
   ```

   **Expected Output** (on failure):

   ```
   Internal Server Error
   ```

   **Server Logs** (example):

   ```
   {"time":"2025-04-15T12:02:00Z","level":"INFO","msg":"Processing request","method":"GET","path":"/hello","trace_id":"ghi789","span_id":"jkl012"}
   {"time":"2025-04-15T12:02:00Z","level":"ERROR","msg":"Request failed","error":"simulated request failure","trace_id":"ghi789","span_id":"jkl012"}
   ```

   This indicates a failed request, with metrics recorded for failed requests and an error trace sent to OpenObserver.

3. **Multiple Requests to Observe Metrics**:

   Run multiple requests to generate varied metrics:

   ```bash
   for i in {1..10}; do curl http://localhost:8080/hello; done
   ```

   This sends 10 requests, some of which may fail, allowing you to observe both success and failure metrics in OpenObserver.

## Verifying in OpenObserver

Log in to your OpenObserver dashboard to verify:

- **Traces**: Look for spans named `handleHello` under `example-service`. Check for success (`OK`) or error statuses.
- **Metrics**:
    - `example_service_module_requests_accepted_total`: Count of successful requests.
    - `example_service_module_requests_failed_total`: Count of failed requests.
    - `example_service_module_request_duration_seconds`: Latency histogram.
- **Logs**: Search for logs with `Processing request` or `Request failed`, including `trace_id` and `span_id` for correlation.

## Stopping the Server

Press `Ctrl+C` to stop the server. The server will shut down gracefully:

```
{"time":"2025-04-15T12:05:00Z","level":"INFO","msg":"Shutting down server..."}
{"time":"2025-04-15T12:05:00Z","level":"INFO","msg":"Server stopped"}
```

## Troubleshooting

- **Connection Errors**: Ensure the `Host` and `Token` in `otel.Config` are correct and that your OpenObserver instance is accessible.
- **No Logs/Metrics/Traces**: Verify the `Organization` and `StreamName` match your OpenObserver setup. Check network connectivity to the endpoint.
- **Dependency Issues**: Run `go mod tidy` and ensure you're using a compatible Go version.

## Notes

- The example uses `slog` for structured logging, integrated with OpenTelemetry via a custom handler.
- Metrics and traces include attributes like `endpoint="/hello"` for filtering in OpenObserver.
- The failure rate (10%) is simulated for demonstration; adjust in `example.go` if needed.

