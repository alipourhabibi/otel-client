package otel

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Config holds configuration parameters for Otel initialization
type Config struct {
	Host         string
	Token        string
	ServiceName  string
	Environment  string
	Organization string
	StreamName   string
	SampleRate   float64 // Sampling rate for traces (0 to 1; 0 disables sampling)
}

// Otel encapsulates OpenTelemetry providers
type Otel struct {
	config Config
	logger *sdklog.LoggerProvider
	meter  *sdkmetric.MeterProvider
	tracer *sdktrace.TracerProvider
}

// New creates and initializes a new Otel instance with the provided configuration
func New(config Config) *Otel {
	return &Otel{
		config: config,
	}
}

// Setup initializes all OpenTelemetry providers
func (o *Otel) Setup(ctx context.Context) error {
	// Initialize logger provider
	logger, err := o.initLoggerProvider(ctx)
	if err != nil {
		return err
	}
	o.logger = logger
	global.SetLoggerProvider(logger)

	// Initialize meter provider
	meter, err := o.initMeterProvider(ctx)
	if err != nil {
		return err
	}
	o.meter = meter
	otel.SetMeterProvider(meter)

	// Initialize tracer provider
	tracer, err := o.initTracerProvider(ctx)
	if err != nil {
		return err
	}
	o.tracer = tracer
	otel.SetTracerProvider(tracer)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// Shutdown gracefully shuts down all providers
func (o *Otel) Shutdown(ctx context.Context) error {
	var errs []error
	if o.logger != nil {
		if err := o.logger.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if o.meter != nil {
		if err := o.meter.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if o.tracer != nil {
		if err := o.tracer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// GetTracerProvider returns the tracer provider
func (o *Otel) GetTracerProvider() *sdktrace.TracerProvider {
	return o.tracer
}

// GetMeterProvider returns the meter provider
func (o *Otel) GetMeterProvider() *sdkmetric.MeterProvider {
	return o.meter
}

// commonHeaders returns the common headers for OTLP exporters
func (o *Otel) commonHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Basic " + o.config.Token,
		"organization":  o.config.Organization,
		"stream-name":   o.config.StreamName,
	}
}

// commonResource creates a common resource configuration
func (o *Otel) commonResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(o.config.ServiceName),
			semconv.DeploymentEnvironment(o.config.Environment),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
}

// initLoggerProvider initializes the logger provider
func (o *Otel) initLoggerProvider(ctx context.Context) (*sdklog.LoggerProvider, error) {
	exporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(o.config.Host),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(o.commonHeaders()),
	)
	if err != nil {
		return nil, err
	}

	res, err := o.commonResource(ctx)
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	), nil
}

// initMeterProvider initializes the meter provider
func (o *Otel) initMeterProvider(ctx context.Context) (*sdkmetric.MeterProvider, error) {
	res, err := o.commonResource(ctx)
	if err != nil {
		return nil, err
	}

	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(o.config.Host),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(o.commonHeaders()),
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

	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(10*time.Second),
		sdkmetric.WithTimeout(5*time.Second),
	)

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	), nil
}

// initTracerProvider initializes the tracer provider
func (o *Otel) initTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	res, err := o.commonResource(ctx)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(o.config.Host),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithHeaders(o.commonHeaders()),
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

	sampler := sdktrace.AlwaysSample()
	if o.config.SampleRate > 0 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(o.config.SampleRate))
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	), nil
}
