# otel-client

Minimal OpenTelemetry bootstrap for Go — one import, consistent traces, metrics, and logs over OTLP/gRPC across all your services.

## Installation

```bash
go get github.com/alipourhabibi/otel-client
```

---

## Usage

```go
client := otel.New(otel.Config{
    Host:        "localhost:4317",
    Token:       os.Getenv("OTEL_TOKEN"),
    ServiceName: "my-service",
    Environment: "production",
    SampleRate:  0.1,
})

if err := client.Setup(ctx); err != nil {
    log.Fatal(err)
}
defer func() {
    sdCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    client.Shutdown(sdCtx)
}()
```

---

## Configuration

```go
type Config struct {
    Host         string  // OTLP gRPC endpoint, e.g. "localhost:4317"
    Token        string  // Sent as "Authorization: Bearer <Token>"
    ServiceName  string  // Stamped on every trace, metric, and log
    Environment  string  // e.g. "production", "staging", "development"
    Organization string  // Custom OTLP header
    StreamName   string  // Custom OTLP header
    SampleRate   float64 // 0=off  0<r<1=ratio  >=1=always
}
```

`Host` and `ServiceName` are required. `SampleRate` must be in [0, 1]. Validation runs automatically on `Setup()`, or call `cfg.Validate()` early to fail before any network dials.

---

## Tracing

```go
tracer := client.TracerProvider().Tracer("github.com/yourorg/yourservice")

ctx, span := tracer.Start(ctx, "operation-name",
    oteltrace.WithSpanKind(oteltrace.SpanKindServer),
    oteltrace.WithAttributes(attribute.String("key", "value")),
)
defer span.End()

// on error
span.RecordError(err)
span.SetStatus(codes.Error, err.Error())

// on success
span.SetStatus(codes.Ok, "")
```

### Messaging

```go
// Producer
ctx, span := tracer.Start(ctx, "queue.publish",
    oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
)
otel.GetTextMapPropagator().Inject(ctx, headers)
defer span.End()

// Consumer
ctx = otel.GetTextMapPropagator().Extract(context.Background(), headers)
ctx, span = tracer.Start(ctx, "queue.process",
    oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
)
defer span.End()
```

---

## Metrics

```go
metrics, err := otel.NewMetricsRecorder(client.MeterProvider(), "payments")

attrs := []attribute.KeyValue{attribute.String("endpoint", "/checkout")}

start := time.Now()
defer metrics.RecordLatency(ctx, time.Since(start), attrs...)

if err != nil {
    metrics.RecordFailed(ctx, attrs...)
} else {
    metrics.RecordAccepted(ctx, attrs...)
}
```

Generated metrics (prefix = component name passed to `NewMetricsRecorder`):

| Metric | Type |
|--------|------|
| `{component}_requests_accepted_total` | Counter |
| `{component}_requests_failed_total` | Counter |
| `{component}_request_duration_seconds` | Histogram |

---

## Logging

```go
base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level:     slog.LevelDebug,
    AddSource: true, // required for source file:line injection
})
slog.SetDefault(slog.New(otel.NewSlogHandler(base, "github.com/yourorg/svc")))

// always pass ctx to get trace_id and span_id
slog.InfoContext(ctx, "user signed in", "user_id", 42)
slog.ErrorContext(ctx, "query failed", "error", err)
```

---

## License

MIT
