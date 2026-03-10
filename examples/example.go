package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/alipourhabibi/otel-client/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg := otel.Config{
		Host:         getEnv("OTEL_HOST", "localhost:4317"),
		Token:        getEnv("OTEL_TOKEN", ""),
		ServiceName:  getEnv("OTEL_SERVICE_NAME", "example-service"),
		Environment:  getEnv("OTEL_ENVIRONMENT", "development"),
		Organization: getEnv("OTEL_ORGANIZATION", ""),
		StreamName:   getEnv("OTEL_STREAM_NAME", ""),
		SampleRate:   1.0,
	}

	client := otel.New(cfg)
	if err := client.Setup(ctx); err != nil {
		slog.Error("otel setup failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		// Fresh context: the signal context is already cancelled here.
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sdCancel()
		if err := client.Shutdown(sdCtx); err != nil {
			slog.Error("otel shutdown failed", "error", err)
		}
	}()

	slog.SetDefault(slog.New(otel.NewLogHandler(cfg.ServiceName)))

	slog.InfoContext(ctx, "service started",
		"service", cfg.ServiceName,
		"env", cfg.Environment,
	)

	metrics, err := otel.NewMetricsRecorder(client.MeterProvider(), cfg.ServiceName, "test-component")
	if err != nil {
		slog.Error("failed to create metrics recorder", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", handleHello(client, metrics))
	mux.HandleFunc("/health", handleHealth())

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	httpCtx, httpCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer httpCancel()
	if err := srv.Shutdown(httpCtx); err != nil {
		slog.Error("http shutdown failed", "error", err)
	}
}

func handleHello(client *otel.Client, metrics *otel.MetricsRecorder) http.HandlerFunc {
	tracer := client.TracerProvider().Tracer("github.com/alipourhabibi/otel-client/examples")

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "handleHello",
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", "/hello"),
			),
		)
		defer span.End()

		start := time.Now()
		reqAttrs := []attribute.KeyValue{
			attribute.String("endpoint", "/hello"),
			attribute.String("method", r.Method),
		}
		defer func() { metrics.RecordLatency(ctx, time.Since(start), reqAttrs...) }()

		slog.InfoContext(ctx, "handling request",
			"method", r.Method,
			"remote_addr", r.RemoteAddr,
		)

		time.Sleep(50 * time.Millisecond)

		// Simulate ~10 % error rate.
		if time.Now().UnixNano()%10 == 0 {
			err := errors.New("simulated failure")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			slog.ErrorContext(ctx, "request failed", "error", err)
			metrics.RecordFailed(ctx, reqAttrs...)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		span.SetStatus(codes.Ok, "")
		metrics.RecordAccepted(ctx, reqAttrs...)
		slog.InfoContext(ctx, "request succeeded")

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("Hello, World!\n"))
	}
}

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
