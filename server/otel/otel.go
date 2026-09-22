/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	otelchimetric "github.com/riandyrn/otelchi/metric"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"github.com/immanent-tech/go-base/config"
)

var TracerProvider *trace.TracerProvider
var MeterProvider *metric.MeterProvider
var MeterConfig otelchimetric.BaseConfig
var enabled atomic.Bool

func init() {
	enabled.Store(false)
}

func IsEnabled() bool {
	return enabled.Load()
}

// Setup bootstraps the OpenTelemetry pipeline. If it does not return an error, make sure to call shutdown for proper
// cleanup.
func Setup(
	ctx context.Context,
	appCfg *config.AppConfig,
) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	// shutdown calls cleanup functions registered via shutdownFuncs. The errors from the calls are joined. Each
	// registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// fail is a helper for the setup-failed path: it runs whatever cleanup has been registered so far and returns a nil
	// shutdown func, since the caller has nothing left to clean up themselves.
	fail := func(err error) (func(context.Context) error, error) {
		return nil, errors.Join(err, shutdown(ctx))
	}

	res, err := newResource(ctx, appCfg)
	if err != nil {
		return fail(fmt.Errorf("build resource: %w", err))
	}

	// Configure Context Propagation to use the default W3C traceparent format.
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	texporter, mreader, err := newExporters(ctx, appCfg.GetAppEnvironment())
	if err != nil {
		return fail(err)
	}

	// Configure Trace Export.
	TracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(texporter),
		trace.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, TracerProvider.Shutdown)
	otel.SetTracerProvider(TracerProvider)

	// Configure Metric Export.
	MeterProvider = metric.NewMeterProvider(
		metric.WithReader(mreader),
		metric.WithResource(res),
	)
	MeterConfig = otelchimetric.NewBaseConfig("foragd", otelchimetric.WithMeterProvider(MeterProvider))
	shutdownFuncs = append(shutdownFuncs, MeterProvider.Shutdown)
	otel.SetMeterProvider(MeterProvider)

	enabled.Store(true)
	slogctx.FromCtx(ctx).Debug("Open Telemetry instrumentation is enabled.")

	return shutdown, nil
}

// newResource builds the OTel resource describing this service, so that spans/metrics show up correctly labeled
// (service.name, host, process, SDK info, etc.) in whatever backend receives them.
func newResource(ctx context.Context, appCfg *config.AppConfig) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("foragd"),
			semconv.ServiceVersion(appCfg.GetAppVersion()),
			semconv.DeploymentEnvironment(appCfg.GetAppEnvironment().String()),
		),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithTelemetrySDK(),
	)
}

// newExporters builds the span exporter and metric reader for the current environment.
func newExporters(ctx context.Context, environment config.Environment) (trace.SpanExporter, metric.Reader, error) {
	if environment == config.EnvProduction {
		texporter, err := autoexport.NewSpanExporter(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("new span exporter: %w", err)
		}
		mreader, err := autoexport.NewMetricReader(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("new metric reader: %w", err)
		}
		return texporter, mreader, nil
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return nil, nil, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT must be set in production")
	}

	tokenSource, err := idtoken.NewTokenSource(ctx, endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("new token source: %w", err)
	}
	// Wrap with ReuseTokenSource so the underlying token is cached and only refreshed once it's near expiry, rather
	// than minting a new one on every single call.
	tokenSource = oauth2.ReuseTokenSource(nil, tokenSource)

	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: tokenSource}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("dial otlp endpoint: %w", err)
	}

	texporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, fmt.Errorf("new otlp trace exporter: %w", err)
	}

	mexporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, fmt.Errorf("new otlp metric exporter: %w", err)
	}
	mreader := metric.NewPeriodicReader(mexporter)

	return texporter, mreader, nil
}
