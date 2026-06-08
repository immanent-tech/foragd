// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package otel

import (
	"context"
	"errors"
	"fmt"
	"os"

	otelchimetric "github.com/riandyrn/otelchi/metric"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/api/idtoken"

	"github.com/immanent-tech/foragd/config"
)

var TracerProvider *trace.TracerProvider
var MeterProvider *metric.MeterProvider
var MeterConfig otelchimetric.BaseConfig

// Setup bootstraps the OpenTelemetry pipeline. If it does not return an error, make sure to call shutdown for proper
// cleanup.
func Setup(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	tokenSource, err := idtoken.NewTokenSource(ctx, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		return nil, fmt.Errorf("new authorization: %w", err)
	}
	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("get authorization token: %w", err)
	}
	if err := os.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization="+token.AccessToken); err != nil {
		return nil, fmt.Errorf("set OTEL_EXPORTER_OTLP_HEADERS: %w", err)
	}

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

	// Configure Context Propagation to use the default W3C traceparent format
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	// Configure Trace Export to send spans as OTLP
	texporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return shutdown, err
	}
	TracerProvider = trace.NewTracerProvider(trace.WithBatcher(texporter))
	shutdownFuncs = append(shutdownFuncs, TracerProvider.Shutdown)
	otel.SetTracerProvider(TracerProvider)

	// Configure Metric Export to send metrics as OTLP
	mreader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return shutdown, err
	}
	MeterProvider = metric.NewMeterProvider(
		metric.WithReader(mreader),
	)
	MeterConfig = otelchimetric.NewBaseConfig(config.AppName, otelchimetric.WithMeterProvider(MeterProvider))
	shutdownFuncs = append(shutdownFuncs, MeterProvider.Shutdown)
	otel.SetMeterProvider(MeterProvider)
	return shutdown, nil
}
