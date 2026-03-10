package otel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds all configuration needed to bootstrap the three OTel signals.
type Config struct {
	// Host is the OTLP gRPC endpoint without scheme, e.g. "localhost:4317".
	Host string

	// Token is sent as "Authorization: <Token>".
	// If your Backend uses Bearer you should add it in Token
	// Leave empty if your collector needs no auth.
	Token string

	// ServiceName is stamped on every trace, metric, and log record.
	ServiceName string

	// Environment is the deployment tier, e.g. "production", "staging".
	Environment string

	// Organization and StreamName are forwarded as custom OTLP headers.
	// Remove these if your backend does not use them.
	Organization string
	StreamName   string

	// SampleRate controls trace sampling:
	//   0        → NeverSample  (disable tracing)
	//   0 < r < 1 → ParentBased(TraceIDRatioBased(r))
	//   >= 1     → AlwaysSample (100%, suitable for dev/staging)
	SampleRate float64
}

// Validate returns a combined error for every invalid field.
// Setup calls this automatically; you may also call it early during
// flag/env parsing to surface problems before any network dials happen.
func (c Config) Validate() error {
	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("otel: Host is required"))
	}
	if c.ServiceName == "" {
		errs = append(errs, errors.New("otel: ServiceName is required"))
	}
	if c.SampleRate < 0 || c.SampleRate > 1 {
		errs = append(errs, fmt.Errorf("otel: SampleRate %.2f is out of range [0, 1]", c.SampleRate))
	}
	return errors.Join(errs...)
}

// Client bootstraps and owns the three OTel SDK providers.
// Create one per process; share it across the application.
type Client struct {
	config Config
	log    *sdklog.LoggerProvider
	meter  *sdkmetric.MeterProvider
	tracer *sdktrace.TracerProvider
}

// New returns a Client. No network connections are made until Setup is called.
func New(config Config) *Client {
	return &Client{config: config}
}

// Setup validates config, builds a shared resource, initialises the log,
// metric, and trace providers, then registers them as the global OTel providers.
//
// On success always pair with a deferred Shutdown - even on later errors —
// to guarantee buffered telemetry is flushed before the process exits.
func (c *Client) Setup(ctx context.Context) error {
	if err := c.config.Validate(); err != nil {
		return err
	}

	// Build the resource once. All three providers share the same instance so
	// service metadata is identical across every signal in your backend.
	res, err := c.buildResource(ctx)
	if err != nil {
		return fmt.Errorf("otel: build resource: %w", err)
	}

	if c.log, err = c.newLogProvider(ctx, res); err != nil {
		return fmt.Errorf("otel: log provider: %w", err)
	}
	global.SetLoggerProvider(c.log)

	if c.meter, err = c.newMeterProvider(ctx, res); err != nil {
		return fmt.Errorf("otel: meter provider: %w", err)
	}
	otel.SetMeterProvider(c.meter)

	if c.tracer, err = c.newTracerProvider(ctx, res); err != nil {
		return fmt.Errorf("otel: tracer provider: %w", err)
	}
	otel.SetTracerProvider(c.tracer)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// Shutdown flushes all in-flight data and closes every provider.
// Use a fresh context with a generous timeout (≥10 s) — do NOT reuse the
// signal context, which is already cancelled by the time shutdown runs.
func (c *Client) Shutdown(ctx context.Context) error {
	var errs []error
	if c.log != nil {
		if err := c.log.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("log: %w", err))
		}
	}
	if c.meter != nil {
		if err := c.meter.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter: %w", err))
		}
	}
	if c.tracer != nil {
		if err := c.tracer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer: %w", err))
		}
	}
	return errors.Join(errs...)
}

// TracerProvider returns the OTel TracerProvider interface.
func (c *Client) TracerProvider() trace.TracerProvider {
	return c.tracer
}

// MeterProvider returns the OTel MeterProvider interface.
func (c *Client) MeterProvider() metric.MeterProvider {
	return c.meter
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"Authorization": c.config.Token,
		"organization":  c.config.Organization,
		"stream-name":   c.config.StreamName,
	}
}

func (c *Client) buildResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(c.config.ServiceName),
			semconv.DeploymentEnvironment(c.config.Environment),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
}

func (c *Client) newLogProvider(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exp, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(c.config.Host),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(c.headers()),
	)
	if err != nil {
		return nil, err
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp,
			sdklog.WithExportInterval(1*time.Second),
			sdklog.WithExportTimeout(5*time.Second),
			sdklog.WithMaxQueueSize(2048),
		)),
	), nil
}

func (c *Client) newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(c.config.Host),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(c.headers()),
		otlpmetricgrpc.WithTimeout(5*time.Second),
		otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     10 * time.Second,
			MaxElapsedTime:  30 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(10*time.Second),
			sdkmetric.WithTimeout(5*time.Second),
		)),
	), nil
}

func (c *Client) newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(c.config.Host),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithHeaders(c.headers()),
		otlptracegrpc.WithTimeout(5*time.Second),
		otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     10 * time.Second,
			MaxElapsedTime:  30 * time.Second,
		}),
	)
	if err != nil {
		return nil, err
	}

	var sampler sdktrace.Sampler
	switch {
	case c.config.SampleRate <= 0:
		sampler = sdktrace.NeverSample()
	case c.config.SampleRate >= 1:
		sampler = sdktrace.AlwaysSample()
	default:
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(c.config.SampleRate))
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	), nil
}
