package otel

import (
	"context"
	"log/slog"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

// otelHandler implements slog.Handler and emits logs to OTEL + slog output
type otelHandler struct {
	otelLogger log.Logger
	logger     *slog.Logger
	attrs      []slog.Attr
	group      string
}

// NewOtelHandler creates a new handler
func NewOtelHandler(l *slog.Logger, name string) slog.Handler {
	return &otelHandler{
		otelLogger: global.GetLoggerProvider().Logger(name),
		logger:     l,
	}
}

// Enabled always returns true
func (h *otelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

// Handle emits the log record to OTEL and slog output
func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := make([]log.KeyValue, 0, len(h.attrs)+r.NumAttrs()+4) // for trace_id and span_id
	logAttrs := make([]any, 0, len(h.attrs)*2+r.NumAttrs()*2+4)

	// handler-level attributes
	for _, a := range h.attrs {
		attrs = append(attrs, log.String(a.Key, a.Value.String()))
		logAttrs = append(logAttrs, a.Key, a.Value.Any())
	}

	// record-level attributes
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, log.String(a.Key, a.Value.String()))
		logAttrs = append(logAttrs, a.Key, a.Value.Any())
		return true
	})

	// group
	if h.group != "" {
		attrs = append(attrs, log.String("group", h.group))
		logAttrs = append(logAttrs, "group", h.group)
	}

	// include span info if present
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		spanCtx := span.SpanContext()
		attrs = append(attrs,
			log.String("trace_id", spanCtx.TraceID().String()),
			log.String("span_id", spanCtx.SpanID().String()),
		)
		logAttrs = append(logAttrs, "trace_id", spanCtx.TraceID().String(), "span_id", spanCtx.SpanID().String())
	}

	// add source file:line
	if pc, file, line, ok := runtime.Caller(3); ok {
		_ = runtime.FuncForPC(pc)
		source := file + ":" + strconv.Itoa(line)
		attrs = append(attrs, log.String("source", source))
		logAttrs = append(logAttrs, "source", source)
	}

	// map slog.Level to OTEL severity
	severity := log.SeverityInfo
	switch r.Level {
	case slog.LevelDebug:
		severity = log.SeverityDebug
	case slog.LevelWarn:
		severity = log.SeverityWarn
	case slog.LevelError:
		severity = log.SeverityError
	}

	// build OTEL log record
	logRecord := log.Record{}
	logRecord.SetSeverity(severity)
	logRecord.SetTimestamp(r.Time)
	logRecord.SetObservedTimestamp(time.Now())
	logRecord.SetBody(log.StringValue(r.Message))
	logRecord.AddAttributes(attrs...)
	logRecord.SetSeverityText(severity.String())

	h.otelLogger.Emit(ctx, logRecord)

	// emit to terminal logger
	h.logger.Log(ctx, r.Level, r.Message, logAttrs...)
	return nil
}

// WithAttrs returns a new handler with additional attributes
func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr(nil), h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &otelHandler{
		otelLogger: h.otelLogger,
		logger:     h.logger,
		attrs:      newAttrs,
		group:      h.group,
	}
}

// WithGroup returns a new handler with group set
func (h *otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{
		otelLogger: h.otelLogger,
		logger:     h.logger,
		attrs:      h.attrs,
		group:      name,
	}
}
