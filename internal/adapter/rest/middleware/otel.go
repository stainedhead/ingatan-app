package middleware

import (
	"context"
	"io"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// OTelConfig holds OpenTelemetry provider configuration.
type OTelConfig struct {
	// Endpoint selects the exporter:
	//   "stdout" — human-readable output for development
	//   ""       — noop provider (no telemetry)
	//   other    — OTLP gRPC endpoint (reserved for future use)
	Endpoint    string
	ServiceName string
}

// OTelProvider holds the configured TracerProvider and MeterProvider
// and implements io.Closer for graceful shutdown.
type OTelProvider struct {
	tracerProvider trace.TracerProvider
	shutdown       func(context.Context) error
}

// Shutdown flushes and stops the OTel providers.
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	return p.shutdown(ctx)
}

// Tracer returns a named tracer from the provider's TracerProvider.
func (p *OTelProvider) Tracer(name string) trace.Tracer {
	return p.tracerProvider.Tracer(name)
}

// NewOTelProvider initialises the global OTel TracerProvider and MeterProvider
// based on cfg. Call provider.Shutdown(ctx) at server exit.
func NewOTelProvider(cfg OTelConfig) (*OTelProvider, error) {
	switch cfg.Endpoint {
	case "":
		// Noop — set noop providers so instrumented code doesn't panic.
		noopTP := noop.NewTracerProvider()
		otel.SetTracerProvider(noopTP)
		return &OTelProvider{
			tracerProvider: noopTP,
			shutdown:       func(_ context.Context) error { return nil },
		}, nil

	case "stdout":
		return newStdoutProvider(cfg)

	default:
		// Future: OTLP gRPC. For now, fall back to noop.
		noopTP := noop.NewTracerProvider()
		otel.SetTracerProvider(noopTP)
		return &OTelProvider{
			tracerProvider: noopTP,
			shutdown:       func(_ context.Context) error { return nil },
		}, nil
	}
}

// newStdoutProvider creates trace + metric providers that write to stdout.
func newStdoutProvider(cfg OTelConfig) (*OTelProvider, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, err
	}

	// Trace exporter — write to stdout (discard output in tests via io.Discard).
	traceExp, err := stdouttrace.New(stdouttrace.WithWriter(io.Discard))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter — write to stdout (discard output in tests).
	metricExp, err := stdoutmetric.New(stdoutmetric.WithWriter(io.Discard))
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp)),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		return mp.Shutdown(ctx)
	}

	return &OTelProvider{tracerProvider: tp, shutdown: shutdown}, nil
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader captures the status code before delegating.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// OTelMiddleware wraps each HTTP handler with an OTel trace span.
// It records http.method, http.target, http.status_code, and
// sets span status to Error for responses >= 500.
// Principal enrichment (principal.id) is handled separately by PrincipalEnrichSpan,
// which must be applied after JWTMiddleware so the principal is in context.
func OTelMiddleware(tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
			)

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r.WithContext(ctx))

			span.SetAttributes(attribute.Int("http.status_code", rw.statusCode))
			if rw.statusCode >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(rw.statusCode))
			}
		})
	}
}

// PrincipalEnrichSpan is applied after JWTMiddleware inside /api/v1.
// It reads the principal from context and adds principal.id to the active span.
func PrincipalEnrichSpan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := PrincipalFromContext(r.Context()); p != nil {
			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(attribute.String("principal.id", p.ID))
		}
		next.ServeHTTP(w, r)
	})
}
