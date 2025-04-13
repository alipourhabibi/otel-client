package otel

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

type otelHandler struct {
	otelLogger log.Logger
	logger     *slog.Logger
}

func NewOtelHandler(l *slog.Logger) slog.Handler {
	return &otelHandler{
		otelLogger: global.GetLoggerProvider().Logger("doctor"),
		logger:     l,
	}
}

func (h *otelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
	severity := log.SeverityInfo
	switch r.Level {
	case slog.LevelDebug:
		severity = log.SeverityDebug
	case slog.LevelWarn:
		severity = log.SeverityWarn
	case slog.LevelError:
		severity = log.SeverityError
	}

	attrs := make([]log.KeyValue, 0, r.NumAttrs())
	logAttrs := make([]any, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, log.String(a.Key, a.Value.String()))
		logAttrs = append(logAttrs, a.Key)
		logAttrs = append(logAttrs, a.Value)
		return true
	})

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		spanCtx := span.SpanContext()
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
	}

	logRecord := log.Record{}
	logRecord.SetSeverity(severity)
	logRecord.SetTimestamp(r.Time)
	logRecord.SetObservedTimestamp(time.Now())
	logRecord.SetBody(log.StringValue(r.Message))
	logRecord.AddAttributes(attrs...)
	logRecord.SetSeverityText(severity.String())

	// send to otel
	h.otelLogger.Emit(ctx, logRecord)

	// our own logger
	h.logger.Log(ctx, r.Level, r.Message, logAttrs...)

	return nil
}

func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *otelHandler) WithGroup(name string) slog.Handler {
	return h
}
