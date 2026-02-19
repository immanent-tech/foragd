// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package otel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport/version"
	otelchimetric "github.com/riandyrn/otelchi/metric"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"

	"github.com/immanent-tech/foragd/config"
)

var TracerProvider *trace.TracerProvider
var MeterProvider *metric.MeterProvider
var MeterConfig otelchimetric.BaseConfig
var LoggerProvider *log.LoggerProvider

// Setup bootstraps the OpenTelemetry pipeline. If it does not return an error, make sure to call shutdown for proper
// cleanup.
func Setup(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}
	otel.SetErrorHandler(&errorHandler{entry: slogctx.FromCtx(ctx).With(slog.String("from", "opentelemetry"))})

	// Set up logger provider.
	loggerProvider, err := newLoggerProvider(ctx)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	LoggerProvider = loggerProvider
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)

	// Set up resources.
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceVersionKey.String(version.Version),
		),
	)

	// Set up trace exporter.
	tracerProvider, err := newTracerProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	TracerProvider = tracerProvider
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)

	// Set up meter provider.
	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	MeterProvider = meterProvider
	MeterConfig = otelchimetric.NewBaseConfig(config.AppName, otelchimetric.WithMeterProvider(MeterProvider))
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)

	return shutdown, nil
}

func newLoggerProvider(ctx context.Context) (*log.LoggerProvider, error) {
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("new grpc logger: %w", err)
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)

	global.SetLoggerProvider(loggerProvider)

	return loggerProvider, nil
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*trace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("new grpc tracer: %w", err)
	}
	opts := []trace.TracerProviderOption{
		trace.WithResource(res),
		trace.WithBatcher(traceExporter),
		trace.WithSampler(trace.AlwaysSample()),
	}
	tracerProvider := trace.NewTracerProvider(opts...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	return tracerProvider, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("new grpc meter: %w", err)
	}
	metricReader := metric.NewPeriodicReader(
		metricExporter,
		metric.WithInterval(5*time.Second),
	)
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metricReader),
	)

	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

type errorHandler struct {
	entry *slog.Logger
}

func (h *errorHandler) Handle(err error) {
	h.entry.Warn("otel error", slog.Any("error", err))
}
