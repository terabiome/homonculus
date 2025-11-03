package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

type Telemetry struct {
	tracerProvider *trace.TracerProvider
	meterProvider  *metric.MeterProvider
}

func Initialize(serviceName string) (*Telemetry, error) {
	opts := []stdouttrace.Option{
		stdouttrace.WithPrettyPrint(),
	}

	traceExporter, err := stdouttrace.New(opts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(meterProvider)

	return &Telemetry{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
	}, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.tracerProvider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	if err := t.meterProvider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}

	return nil
}
