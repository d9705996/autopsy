// Package observability bootstraps structured logging (slog) and the
// OpenTelemetry SDK (traces + metrics) for the Autopsy process.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Provider holds the OTel SDK providers and exposes a Shutdown function.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	logger         *slog.Logger
}

// Config controls observability bootstrap behaviour.
type Config struct {
	ServiceName    string
	ServiceVersion string
	LogLevel       string
	LogFormat      string
	OTLPEndpoint   string // empty -> no-op trace exporter
}

// New initialises the OTel SDK and constructs a *slog.Logger.
// Call Shutdown on process exit to flush exporters.
func New(ctx context.Context, cfg *Config) (*Provider, *slog.Logger, error) {
	logger := buildLogger(cfg.LogLevel, cfg.LogFormat)

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("build otel resource: %w", err)
	}

	// --- Tracer provider ---
	var traceOpts []sdktrace.TracerProviderOption
	traceOpts = append(traceOpts, sdktrace.WithResource(res))

	if cfg.OTLPEndpoint != "" {
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("build otlp exporter: %w", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	} else {
		// no-op: traces are dropped when no endpoint is configured
		logger.Debug("otel: no OTLP endpoint configured; traces disabled")
	}

	tp := sdktrace.NewTracerProvider(traceOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// --- Meter provider (Prometheus exporter) ---
	promExp, err := otelprometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("build prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExp),
	)
	otel.SetMeterProvider(mp)

	return &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
		logger:         logger,
	}, logger, nil
}

// Shutdown drains all exporters with a 10-second timeout.
func (p *Provider) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := p.tracerProvider.Shutdown(ctx); err != nil {
		p.logger.Error("otel tracer shutdown", "err", err)
	}
	if err := p.meterProvider.Shutdown(ctx); err != nil {
		p.logger.Error("otel meter shutdown", "err", err)
	}
}

func buildLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
