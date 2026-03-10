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

func NewLogHandler(name string) slog.Handler {
	return &Handler{
		logger: global.GetLoggerProvider().Logger(name),
	}
}

type Handler struct {
	logger log.Logger
	attrs  []slog.Attr
	group  string
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.logger.Enabled(ctx, log.EnabledParameters{
		Severity: slogToOtelSeverity(level),
	})
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	attrCap := len(h.attrs) + r.NumAttrs() + 3

	attrs := make([]log.KeyValue, 0, attrCap)

	appendAttr := func(k string, v slog.Value) {
		switch v.Kind() {

		case slog.KindString:
			attrs = append(attrs, log.String(k, v.String()))

		case slog.KindInt64:
			attrs = append(attrs, log.Int64(k, v.Int64()))

		case slog.KindFloat64:
			attrs = append(attrs, log.Float64(k, v.Float64()))

		case slog.KindBool:
			attrs = append(attrs, log.Bool(k, v.Bool()))

		case slog.KindDuration:
			attrs = append(attrs, log.Int64(k, int64(v.Duration())))

		case slog.KindTime:
			attrs = append(attrs, log.String(k, v.Time().Format(time.RFC3339Nano)))

		default:
			attrs = append(attrs, log.String(k, v.String()))
		}
	}

	for _, a := range h.attrs {
		appendAttr(a.Key, a.Value)
	}

	r.Attrs(func(a slog.Attr) bool {
		appendAttr(a.Key, a.Value)
		return true
	})

	if h.group != "" {
		attrs = append(attrs, log.String("group", h.group))
	}

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()

		attrs = append(attrs,
			log.String("trace_id", sc.TraceID().String()),
			log.String("span_id", sc.SpanID().String()),
		)
	}

	if r.PC != 0 {
		frames := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := frames.Next()

		attrs = append(attrs,
			log.String("source", f.File+":"+strconv.Itoa(f.Line)),
		)
	}

	var rec log.Record

	severity := slogToOtelSeverity(r.Level)

	rec.SetSeverity(severity)
	rec.SetSeverityText(severity.String())
	rec.SetTimestamp(r.Time)
	rec.SetObservedTimestamp(time.Now())
	rec.SetBody(log.StringValue(r.Message))

	rec.AddAttributes(attrs...)

	h.logger.Emit(ctx, rec)

	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))

	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)

	return &Handler{
		logger: h.logger,
		attrs:  merged,
		group:  h.group,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		logger: h.logger,
		attrs:  h.attrs,
		group:  name,
	}
}

func slogToOtelSeverity(l slog.Level) log.Severity {
	switch {
	case l <= slog.LevelDebug:
		return log.SeverityDebug
	case l < slog.LevelWarn:
		return log.SeverityInfo
	case l < slog.LevelError:
		return log.SeverityWarn
	default:
		return log.SeverityError
	}
}
