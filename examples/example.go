package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Dadegostar/otel-client/otel"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	// Initialize context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Configure OpenTelemetry
	config := otel.Config{
		Host:         "localhost:5081",
		Token:        "cm9vdEBleGFtcGxlLmNvbTpDb21wbGV4cGFzcyMxMjM=",
		ServiceName:  "example-service",
		Environment:  "development",
		Organization: "example-org",
		StreamName:   "example-stream",
		SampleRate:   1.0,
	}

	// Initialize OpenTelemetry
	otelClient := otel.New(config)
	if err := otelClient.Setup(ctx); err != nil {
		slog.Error("Failed to setup OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := otelClient.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown OpenTelemetry", "error", err)
		}
	}()

	// Set up slog with OpenTelemetry handler
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(jsonHandler)
	otelLogger := slog.New(otel.NewOtelHandler(logger, config.ServiceName))
	slog.SetDefault(otelLogger)

	// Test log to verify ingestion
	slog.InfoContext(ctx, "Test log from main", "app", "example-service")

	// Brief delay to allow batch processor to flush
	time.Sleep(2 * time.Second)

	// Initialize metrics recorder
	metrics, err := otel.NewMetricsRecorder(otelClient.GetMeterProvider(), "example-service")
	if err != nil {
		slog.Error("Failed to create metrics recorder", "error", err)
		os.Exit(1)
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", handleHello(otelClient, metrics))

	// Start HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		slog.Info("Starting HTTP server on :8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
	slog.Info("Server stopped")
}

func handleHello(otelClient *otel.Otel, metrics *otel.MetricsRecorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Start a new span
		ctx, span := otel.StartSpan(r.Context(), otelClient.GetTracerProvider(), "handleHello")
		defer span.End()

		// Record latency
		start := time.Now()
		defer func() {
			duration := time.Since(start)
			metrics.RecordLatency(ctx, duration, attribute.String("endpoint", "/hello"))
		}()

		// Log request
		slog.InfoContext(ctx, "Processing request", "method", r.Method, "path", r.URL.Path)

		// Simulate some work
		time.Sleep(100 * time.Millisecond)

		// Randomly fail 10% of the time
		if time.Now().UnixNano()%99 == 0 {
			err := errors.New("simulated request failure")
			slog.ErrorContext(ctx, "Request failed", "error", err)
			otel.RecordTraceError(err, "example-service", span)
			metrics.RecordFailedRequest(ctx, attribute.String("endpoint", "/hello"))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Record success
		otel.RecordTraceSuccessful("example-service", span)
		metrics.RecordAcceptedRequest(ctx, attribute.String("endpoint", "/hello"))
		slog.InfoContext(ctx, "Request completed successfully")

		// Respond
		w.Write([]byte("Hello, World!"))
	}
}
