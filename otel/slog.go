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

// NewSlogHandler wraps an existing slog.Handler and fans every log record out to:
//   - the OTLP log pipeline (for export to your observability backend), and
//   - the wrapped handler as-is (for terminal / file output).
//
// Trace and span IDs are automatically injected whenever the context carries
// an active span, giving you log-to-trace correlation for free.
//
//	slog.SetDefault(slog.New(otel.NewSlogHandler(base, "github.com/yourorg/svc")))
func NewSlogHandler(wrapped slog.Handler, name string) slog.Handler {
	return &slogHandler{
		otelLogger: global.GetLoggerProvider().Logger(name),
		wrapped:    wrapped,
	}
}

type slogHandler struct {
	otelLogger log.Logger
	wrapped    slog.Handler
	attrs      []slog.Attr
	group      string
}

// Enabled delegates to the OTEL logger's own level check.
func (h *slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.otelLogger.Enabled(ctx, log.EnabledParameters{
		Severity: slogToOtelSeverity(level),
	})
}

func (h *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	cap := len(h.attrs) + r.NumAttrs() + 4 // +4: group, trace_id, span_id, source
	otelAttrs := make([]log.KeyValue, 0, cap)
	slogAttrs := make([]any, 0, cap*2)

	appendAttr := func(k string, v slog.Value) {
		otelAttrs = append(otelAttrs, log.String(k, v.String()))
		slogAttrs = append(slogAttrs, k, v.Any())
	}

	for _, a := range h.attrs {
		appendAttr(a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(a.Key, a.Value)
		return true
	})
	if h.group != "" {
		appendAttr("group", slog.StringValue(h.group))
	}

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		appendAttr("trace_id", slog.StringValue(sc.TraceID().String()))
		appendAttr("span_id", slog.StringValue(sc.SpanID().String()))
	}

	if r.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := frames.Next()
		appendAttr("source", slog.StringValue(f.File+":"+strconv.Itoa(f.Line)))
	}

	severity := slogToOtelSeverity(r.Level)
	var otelRecord log.Record
	otelRecord.SetSeverity(severity)
	otelRecord.SetSeverityText(severity.String())
	otelRecord.SetTimestamp(r.Time)
	otelRecord.SetObservedTimestamp(time.Now())
	otelRecord.SetBody(log.StringValue(r.Message))
	otelRecord.AddAttributes(otelAttrs...)
	h.otelLogger.Emit(ctx, otelRecord)

	return h.wrapped.Handle(ctx, r.Clone())
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)
	return &slogHandler{
		otelLogger: h.otelLogger,
		wrapped:    h.wrapped.WithAttrs(attrs),
		attrs:      merged,
		group:      h.group,
	}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	return &slogHandler{
		otelLogger: h.otelLogger,
		wrapped:    h.wrapped.WithGroup(name),
		attrs:      h.attrs,
		group:      name,
	}
}

func slogToOtelSeverity(l slog.Level) log.Severity {
	switch l {
	case slog.LevelDebug:
		return log.SeverityDebug
	case slog.LevelWarn:
		return log.SeverityWarn
	case slog.LevelError:
		return log.SeverityError
	default:
		return log.SeverityInfo
	}
}
