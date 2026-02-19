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
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

// Config contains the OTEL providers.
type Config struct {
	Tracer *trace.TracerProvider
	Meter  *metric.MeterProvider
	Log    *log.LoggerProvider
}

// Setup bootstraps the OpenTelemetry pipeline. If it does not return an error, make sure to call shutdown for proper
// cleanup.
func Setup(ctx context.Context) (*Config, func(context.Context) error, error) {
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
		return nil, shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)

	// // Set up propagator.
	// prop := newPropagator()
	// otel.SetTextMapPropagator(prop)

	// Set up resources.
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceVersionKey.String(version.Version),
		),
	)

	// Set up trace exporter.
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		handleErr(err)
		return nil, shutdown, fmt.Errorf("new grpc tracer: %w", err)
	}
	opts := []trace.TracerProviderOption{
		trace.WithResource(res),
		trace.WithBatcher(traceExporter),
	}
	tracerProvider := trace.NewTracerProvider(opts...)
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		handleErr(err)
		return nil, shutdown, fmt.Errorf("new grpc meter: %w", err)
	}
	metricReader := metric.NewPeriodicReader(
		metricExporter,
		metric.WithInterval(5*time.Second),
	)
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metricReader),
	)
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	return &Config{
			Tracer: tracerProvider,
			Meter:  meterProvider,
			Log:    loggerProvider,
		},
		shutdown, nil
}

// func newPropagator() propagation.TextMapPropagator {
// 	return propagation.NewCompositeTextMapPropagator(
// 		propagation.TraceContext{},
// 		propagation.Baggage{},
// 	)
// }

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

type errorHandler struct {
	entry *slog.Logger
}

func (h *errorHandler) Handle(err error) {
	h.entry.Warn("otel error", slog.Any("error", err))
}
