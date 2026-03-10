package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MetricsRecorder struct {
	accepted metric.Int64Counter
	failed   metric.Int64Counter
	latency  metric.Float64Histogram
}

func NewMetricsRecorder(
	mp metric.MeterProvider,
	instrumentationScope string,
	component string,
) (*MetricsRecorder, error) {

	m := mp.Meter(instrumentationScope)

	accepted, err := m.Int64Counter(
		fmt.Sprintf("%s_requests_accepted_total", component),
		metric.WithDescription("Total accepted requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create accepted counter: %w", err)
	}

	failed, err := m.Int64Counter(
		fmt.Sprintf("%s_requests_failed_total", component),
		metric.WithDescription("Total failed requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create failed counter: %w", err)
	}

	latency, err := m.Float64Histogram(
		fmt.Sprintf("%s_request_duration_seconds", component),
		metric.WithDescription("Request latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create latency histogram: %w", err)
	}

	return &MetricsRecorder{
		accepted: accepted,
		failed:   failed,
		latency:  latency,
	}, nil
}

func (r *MetricsRecorder) RecordAccepted(ctx context.Context, attrs ...attribute.KeyValue) {
	r.accepted.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *MetricsRecorder) RecordFailed(ctx context.Context, attrs ...attribute.KeyValue) {
	r.failed.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (r *MetricsRecorder) RecordLatency(ctx context.Context, d time.Duration, attrs ...attribute.KeyValue) {
	r.latency.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}
