package otel

import (
	"context"
	"log"
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

type Otel struct {
	otelHost    string
	otelToken   string
	environment string
	serviceName string
	LP          *sdklog.LoggerProvider
	MP          *sdkmetric.MeterProvider
	TP          *sdktrace.TracerProvider
}

func NewOtel(otelHost, otelToken, serviceName, environment string) *Otel {
	return &Otel{
		otelHost:    otelHost,
		otelToken:   otelToken,
		serviceName: serviceName,
		environment: environment,
	}
}

func (o *Otel) Setup(ctx context.Context) error {
	lp, err := o.initLog(ctx)
	if err != nil {
		return err
	}
	o.LP = lp

	mp, err := o.initMetricProvider(ctx)
	if err != nil {
		return err
	}
	o.MP = mp

	tp, err := o.initTraceProvider(ctx)
	if err != nil {
		return err
	}
	o.TP = tp
	return nil
}

// init the logger and set it to global default
func (o *Otel) initLog(ctx context.Context) (*sdklog.LoggerProvider, error) {

	exporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(o.otelHost),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(map[string]string{
			"Authorization": "Basic " + o.otelToken,
			"organization":  "default",
			"stream-name":   "default",
		}),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("doctor"),
		),
	)
	if err != nil {
		return nil, err
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(exporter),
		),
	)

	global.SetLoggerProvider(loggerProvider)
	return loggerProvider, nil
}

func (s *Otel) initMetricProvider(ctx context.Context) (*sdkmetric.MeterProvider, error) {

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("doctor"),
			semconv.DeploymentEnvironment(s.environment),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	exporterMetric, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(map[string]string{
			"Authorization": "Basic " + s.otelToken,
			"organization":  "default",
			"stream-name":   "default",
		}),
		otlpmetricgrpc.WithEndpoint(s.otelHost),
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

	metricReader := sdkmetric.NewPeriodicReader(
		exporterMetric,
		sdkmetric.WithInterval(10*time.Second),
		sdkmetric.WithTimeout(5*time.Second),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricReader),
	)

	otel.SetMeterProvider(mp)
	return mp, nil
}

func (s *Otel) initTraceProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("doctor"),
			semconv.DeploymentEnvironment(s.env.Environment),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Basic " + s.otelToken,
			"organization":  "default",
			"stream-name":   "default",
		}),
		otlptracegrpc.WithEndpoint(s.env.OtelEndpoint),
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

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

func (s *Otel) Shutdown(ctx context.Context) {
	if err := s.LP.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down log provider: %v", err)
	}
	if err := s.MP.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down meter provider: %v", err)
	}
	if err := s.TP.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down tracer provider: %v", err)
	}
}
