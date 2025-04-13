package otel

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// MessagingPropagator handles trace context propagation for messaging systems
type MessagingPropagator struct {
	propagator propagation.TextMapPropagator
}

// NewMessagingPropagator creates a new messaging propagator
func NewMessagingPropagator() *MessagingPropagator {
	return &MessagingPropagator{
		propagator: propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}
}

// InjectTraceContext injects trace context into a carrier
func (mp *MessagingPropagator) InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	mp.propagator.Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from a carrier
func (mp *MessagingPropagator) ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return mp.propagator.Extract(ctx, carrier)
}

// StartConsumerSpan starts a span for a message consumer
func (mp *MessagingPropagator) StartConsumerSpan(ctx context.Context, tracerProvider trace.TracerProvider, operationName string) (context.Context, trace.Span) {
	tracer := tracerProvider.Tracer("otel-client")
	return tracer.Start(ctx, operationName, trace.WithSpanKind(trace.SpanKindConsumer))
}

// StartProducerSpan starts a span for a message producer
func (mp *MessagingPropagator) StartProducerSpan(ctx context.Context, tracerProvider trace.TracerProvider, operationName string) (context.Context, trace.Span) {
	tracer := tracerProvider.Tracer("otel-client")
	return tracer.Start(ctx, operationName, trace.WithSpanKind(trace.SpanKindProducer))
}
