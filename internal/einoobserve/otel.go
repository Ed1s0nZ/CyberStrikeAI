package einoobserve

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cyberstrike-ai/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

var (
	otelMu           sync.Mutex
	otelShutdown     func(context.Context) error
	otelInitialized  bool
)

// InitOtelFromConfig installs the global OpenTelemetry TracerProvider when
// eino_callbacks.otel is enabled and exporter is not none. Safe to call multiple times.
func InitOtelFromConfig(cfg *config.MultiAgentEinoCallbacksConfig, log *zap.Logger) (shutdown func(context.Context) error, err error) {
	shutdown = func(context.Context) error { return nil }
	if cfg == nil || !cfg.OtelTracingActive() {
		return shutdown, nil
	}

	otelMu.Lock()
	defer otelMu.Unlock()
	if otelInitialized {
		if otelShutdown != nil {
			return otelShutdown, nil
		}
		return shutdown, nil
	}

	oc := cfg.Otel
	expKind := oc.OtelExporterEffective()
	ctx := context.Background()

	var exporter sdktrace.SpanExporter
	switch expKind {
	case "stdout":
		exporter, err = stdouttrace.New()
		if err != nil {
			return shutdown, fmt.Errorf("eino otel stdout exporter: %w", err)
		}
	case "otlphttp":
		ep := strings.TrimSpace(oc.OTLPEndpoint)
		if ep == "" {
			ep = "localhost:4318"
		}
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(ep),
			otlptracehttp.WithURLPath("/v1/traces"),
		)
		if err != nil {
			return shutdown, fmt.Errorf("eino otel otlphttp exporter: %w", err)
		}
	default:
		return shutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(oc.ServiceNameEffective()),
		),
	)
	if err != nil {
		return shutdown, fmt.Errorf("eino otel resource: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(oc.SampleRatioEffective()))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)

	otelShutdown = tp.Shutdown
	otelInitialized = true
	if log != nil {
		log.Info("eino otel: tracer provider initialized",
			zap.String("exporter", expKind),
			zap.String("service", oc.ServiceNameEffective()),
			zap.Float64("sample_ratio", oc.SampleRatioEffective()),
		)
	}
	return otelShutdown, nil
}

// ShutdownOtel flushes and shuts down the global TracerProvider if it was installed.
func ShutdownOtel(ctx context.Context) error {
	otelMu.Lock()
	fn := otelShutdown
	otelShutdown = nil
	inited := otelInitialized
	otelInitialized = false
	otelMu.Unlock()
	if !inited || fn == nil {
		return nil
	}
	return fn(ctx)
}
