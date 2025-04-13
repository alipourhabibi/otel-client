package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// RecordTraceError records an error in a span
func RecordTraceError(err error, serviceName string, span trace.Span) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent(serviceName, trace.WithAttributes(
		attribute.String("error", err.Error())))
}

// RecordTraceSuccessful records a successful operation in a span
func RecordTraceSuccessful(serviceName string, span trace.Span) {
	span.SetStatus(codes.Ok, "OK")
	span.AddEvent(serviceName + " successful")
}

// StartSpan creates a new span with the given name
func StartSpan(ctx context.Context, tracerProvider trace.TracerProvider, name string) (context.Context, trace.Span) {
	tracer := tracerProvider.Tracer("otel-client")
	return tracer.Start(ctx, name)
}

// MetricsRecorder helps create and record metrics for module or API requests
type MetricsRecorder struct {
	meter            metric.Meter
	acceptedRequests metric.Int64Counter
	failedRequests   metric.Int64Counter
	latency          metric.Float64Histogram
}

// NewMetricsRecorder creates a new metrics recorder for a service
func NewMetricsRecorder(meterProvider metric.MeterProvider, serviceName string) (*MetricsRecorder, error) {
	meter := meterProvider.Meter(serviceName)

	acceptedRequests, err := meter.Int64Counter(
		fmt.Sprintf("%s_module_requests_accepted_total", serviceName),
		metric.WithDescription("Total number of requests accepted by a module or API"),
	)
	if err != nil {
		return nil, err
	}

	failedRequests, err := meter.Int64Counter(
		fmt.Sprintf("%s_module_requests_failed_total", serviceName),
		metric.WithDescription("Total number of requests failed by a module or API"),
	)
	if err != nil {
		return nil, err
	}

	latency, err := meter.Float64Histogram(
		fmt.Sprintf("%s_module_request_duration_seconds", serviceName),
		metric.WithDescription("Request processing latency in seconds for a module or API"),
	)
	if err != nil {
		return nil, err
	}

	return &MetricsRecorder{
		meter:            meter,
		acceptedRequests: acceptedRequests,
		failedRequests:   failedRequests,
		latency:          latency,
	}, nil
}

// RecordAcceptedRequest records a successful request for a module or API
func (m *MetricsRecorder) RecordAcceptedRequest(ctx context.Context, attributes ...attribute.KeyValue) {
	m.acceptedRequests.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordFailedRequest records a failed request for a module or API
func (m *MetricsRecorder) RecordFailedRequest(ctx context.Context, attributes ...attribute.KeyValue) {
	m.failedRequests.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordLatency records request latency for a module or API
func (m *MetricsRecorder) RecordLatency(ctx context.Context, duration time.Duration, attributes ...attribute.KeyValue) {
	m.latency.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
}
