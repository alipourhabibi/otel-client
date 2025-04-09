package otel

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func RecordTraceError(err error, serviceName string, span trace.Span) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.AddEvent(serviceName, trace.WithAttributes(
		attribute.String("error", err.Error())))
}

func RecordTraceSuccessful(serviceName string, span trace.Span) {
	span.SetStatus(codes.Ok, "OK")
	span.AddEvent(serviceName + " successfull")
}
