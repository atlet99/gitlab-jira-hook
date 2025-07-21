package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// HTTP status codes
	httpErrorThreshold = 400
)

// TracingConfig holds configuration for tracing
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	EnableConsole  bool
	SampleRate     float64
}

// Tracer provides distributed tracing capabilities
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	logger   *slog.Logger
	config   *TracingConfig
}

// NewTracer creates a new tracer instance
func NewTracer(config *TracingConfig, logger *slog.Logger) (*Tracer, error) {
	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporters
	var exporters []sdktrace.SpanExporter

	if config.OTLPEndpoint != "" {
		otlpExporter, err := otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		)
		if err != nil {
			logger.Warn("Failed to create OTLP exporter", "error", err)
		} else {
			exporters = append(exporters, otlpExporter)
		}
	}

	if config.EnableConsole {
		consoleExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			logger.Warn("Failed to create console exporter", "error", err)
		} else {
			exporters = append(exporters, consoleExporter)
		}
	}

	if len(exporters) == 0 {
		// Fallback to console exporter if no other exporters are available
		consoleExporter, err := stdouttrace.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create console exporter: %w", err)
		}
		exporters = append(exporters, consoleExporter)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporters[0]),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Create tracer
	tracer := provider.Tracer(config.ServiceName)

	return &Tracer{
		tracer:   tracer,
		provider: provider,
		logger:   logger,
		config:   config,
	}, nil
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string,
	opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes
func (t *Tracer) StartSpanWithAttributes(ctx context.Context, name string,
	attrs map[string]interface{}, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	spanOpts := make([]trace.SpanStartOption, 0, len(opts)+1)
	spanOpts = append(spanOpts, opts...)
	spanOpts = append(spanOpts, trace.WithAttributes(t.convertAttributes(attrs)...))
	return t.tracer.Start(ctx, name, spanOpts...)
}

// TraceHTTPRequest traces an HTTP request
func (t *Tracer) TraceHTTPRequest(ctx context.Context, method, url, userAgent string,
	statusCode int, duration time.Duration) {
	_, span := t.StartSpan(ctx, "http.request")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("http.url", url),
		attribute.String("http.user_agent", userAgent),
		attribute.Int("http.status_code", statusCode),
		attribute.Int64("http.duration_ms", duration.Milliseconds()),
	)

	if statusCode >= httpErrorThreshold {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// TraceJobProcessing traces job processing
func (t *Tracer) TraceJobProcessing(ctx context.Context, jobID, eventType, priority string,
	duration time.Duration, err error) {
	_, span := t.StartSpan(ctx, "job.processing")
	defer span.End()

	span.SetAttributes(
		attribute.String("job.id", jobID),
		attribute.String("job.event_type", eventType),
		attribute.String("job.priority", priority),
		attribute.Int64("job.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// TraceCacheOperation traces cache operations
func (t *Tracer) TraceCacheOperation(ctx context.Context, operation, key, level string,
	hit bool, duration time.Duration) {
	_, span := t.StartSpan(ctx, "cache.operation")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
		attribute.String("cache.level", level),
		attribute.Bool("cache.hit", hit),
		attribute.Int64("cache.duration_ms", duration.Milliseconds()),
	)

	span.SetStatus(codes.Ok, "")
}

// TraceRateLimit traces rate limiting operations
func (t *Tracer) TraceRateLimit(ctx context.Context, endpoint, ip string,
	allowed bool, reason string) {
	_, span := t.StartSpan(ctx, "rate_limit.check")
	defer span.End()

	span.SetAttributes(
		attribute.String("rate_limit.endpoint", endpoint),
		attribute.String("rate_limit.ip", ip),
		attribute.Bool("rate_limit.allowed", allowed),
		attribute.String("rate_limit.reason", reason),
	)

	if !allowed {
		span.SetStatus(codes.Error, "Rate limit exceeded")
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// TraceDatabaseOperation traces database operations
func (t *Tracer) TraceDatabaseOperation(ctx context.Context, operation, table string,
	duration time.Duration, err error) {
	_, span := t.StartSpan(ctx, "database.operation")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// TraceExternalAPI traces external API calls
func (t *Tracer) TraceExternalAPI(ctx context.Context, service, endpoint, method string,
	statusCode int, duration time.Duration, err error) {
	_, span := t.StartSpan(ctx, "external.api.call")
	defer span.End()

	span.SetAttributes(
		attribute.String("external.service", service),
		attribute.String("external.endpoint", endpoint),
		attribute.String("external.method", method),
		attribute.Int("external.status_code", statusCode),
		attribute.Int64("external.duration_ms", duration.Milliseconds()),
	)

	switch {
	case err != nil:
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	case statusCode >= httpErrorThreshold:
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	default:
		span.SetStatus(codes.Ok, "")
	}
}

// TraceAlert traces alert operations
func (t *Tracer) TraceAlert(ctx context.Context, alertID, level, message string,
	triggered bool) {
	_, span := t.StartSpan(ctx, "alert.operation")
	defer span.End()

	span.SetAttributes(
		attribute.String("alert.id", alertID),
		attribute.String("alert.level", level),
		attribute.String("alert.message", message),
		attribute.Bool("alert.triggered", triggered),
	)

	span.SetStatus(codes.Ok, "")
}

// AddEvent adds an event to the current span
func (t *Tracer) AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(t.convertAttributes(attrs)...))
}

// SetAttributes sets attributes on the current span
func (t *Tracer) SetAttributes(ctx context.Context, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(t.convertAttributes(attrs)...)
}

// RecordError records an error on the current span
func (t *Tracer) RecordError(ctx context.Context, err error, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, trace.WithAttributes(t.convertAttributes(attrs)...))
}

// convertAttributes converts map[string]interface{} to []attribute.KeyValue
func (t *Tracer) convertAttributes(attrs map[string]interface{}) []attribute.KeyValue {
	var result []attribute.KeyValue
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			result = append(result, attribute.String(k, val))
		case int:
			result = append(result, attribute.Int(k, val))
		case int64:
			result = append(result, attribute.Int64(k, val))
		case float64:
			result = append(result, attribute.Float64(k, val))
		case bool:
			result = append(result, attribute.Bool(k, val))
		default:
			result = append(result, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}
	return result
}

// Shutdown gracefully shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// GetTracer returns the underlying tracer
func (t *Tracer) GetTracer() trace.Tracer {
	return t.tracer
}

// GetProvider returns the tracer provider
func (t *Tracer) GetProvider() *sdktrace.TracerProvider {
	return t.provider
}

// TracingMiddleware creates a middleware that adds tracing to HTTP requests
func TracingMiddleware(tracer *Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Extract trace context from headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(),
				propagation.HeaderCarrier(r.Header))

			// Start span
			ctx, span := tracer.StartSpan(ctx, "http.request")
			defer span.End()

			// Add request attributes
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			)

			// Create response writer that captures status code
			rw := &ResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}

			// Inject trace context into request
			r = r.WithContext(ctx)

			// Process request
			next.ServeHTTP(rw, r)

			// Record response attributes
			duration := time.Since(start)
			span.SetAttributes(
				attribute.Int("http.status_code", rw.StatusCode),
				attribute.Int64("http.duration_ms", duration.Milliseconds()),
			)

			// Set span status
			if rw.StatusCode >= httpErrorThreshold {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", rw.StatusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}
