// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package otel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"google.golang.org/grpc/credentials"

	"github.com/immanent-tech/foragd/config"
)

type Config struct {
	Tracer *sdktrace.TracerProvider
	Meter  *sdkmetric.MeterProvider
}

func Create(ctx context.Context) (*Config, error) {
	otel.SetErrorHandler(&otelErrorHandler{entry: slogctx.FromCtx(ctx).With(slog.String("source", "opentelemetry"))})

	protocol := "grpc"
	if customProtocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); customProtocol != "" {
		protocol = customProtocol
	}

	var (
		traceExporter  *otlptrace.Exporter
		metricExporter sdkmetric.Exporter
		err            error
	)

	switch protocol {
	case "grpc":
		traceExporter, metricExporter, err = buildGRPCExporters()
	case "http/protobuf", "http", "https":
		traceExporter, metricExporter, err = buildHTTPExporters()
	default:
		return nil, fmt.Errorf("unsupported opentelemetry protocol: %s", protocol)
	}
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	if len(os.Getenv("OTEL_SERVICE_NAME")) == 0 {
		os.Setenv("OTEL_SERVICE_NAME", config.AppName+"/"+config.Version)
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceVersionKey.String(config.Version),
		),
	)

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	}

	tracerProvider := sdktrace.NewTracerProvider(opts...)

	// tracer := tracerProvider.Tracer(config.AppName)

	metricReader := sdkmetric.NewPeriodicReader(
		metricExporter,
		sdkmetric.WithInterval(5*time.Second),
	)

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricReader),
	)

	return &Config{
		Tracer: tracerProvider,
		Meter:  meterProvider,
	}, nil
}

type otelErrorHandler struct {
	entry *slog.Logger
}

func (h *otelErrorHandler) Handle(err error) {
	h.entry.Warn("otel error.",
		slog.Any("error", err),
	)
}

func buildGRPCExporters() (*otlptrace.Exporter, sdkmetric.Exporter, error) {
	tracerOpts := []otlptracegrpc.Option{}
	meterOpts := []otlpmetricgrpc.Option{}

	if tlsConf, err := buildTLSConfig(); tlsConf != nil && err == nil {
		creds := credentials.NewTLS(tlsConf)
		tracerOpts = append(tracerOpts, otlptracegrpc.WithTLSCredentials(creds))
		meterOpts = append(meterOpts, otlpmetricgrpc.WithTLSCredentials(creds))
	} else if err != nil {
		return nil, nil, err
	}

	tracesConnTimeout, metricsConnTimeout, err := getConnectionTimeouts()
	if err != nil {
		return nil, nil, err
	}

	trctx, trcancel := context.WithTimeout(context.Background(), tracesConnTimeout)
	defer trcancel()

	traceExporter, err := otlptracegrpc.New(trctx, tracerOpts...)
	if err != nil {
		err = fmt.Errorf("Can't connect to OpenTelemetry collector: %s", err)
	}

	// if !config.OpenTelemetryEnableMetrics {
	// 	return traceExporter, nil, err
	// }

	mtctx, mtcancel := context.WithTimeout(context.Background(), metricsConnTimeout)
	defer mtcancel()

	metricExporter, err := otlpmetricgrpc.New(mtctx, meterOpts...)
	if err != nil {
		err = fmt.Errorf("Can't connect to OpenTelemetry collector: %s", err)
	}

	return traceExporter, metricExporter, err
}

func buildHTTPExporters() (*otlptrace.Exporter, sdkmetric.Exporter, error) {
	tracerOpts := []otlptracehttp.Option{}
	meterOpts := []otlpmetrichttp.Option{}

	if tlsConf, err := buildTLSConfig(); tlsConf != nil && err == nil {
		tracerOpts = append(tracerOpts, otlptracehttp.WithTLSClientConfig(tlsConf))
		meterOpts = append(meterOpts, otlpmetrichttp.WithTLSClientConfig(tlsConf))
	} else if err != nil {
		return nil, nil, err
	}

	tracesConnTimeout, metricsConnTimeout, err := getConnectionTimeouts()
	if err != nil {
		return nil, nil, err
	}

	trctx, trcancel := context.WithTimeout(context.Background(), tracesConnTimeout)
	defer trcancel()

	traceExporter, err := otlptracehttp.New(trctx, tracerOpts...)
	if err != nil {
		err = fmt.Errorf("Can't connect to OpenTelemetry collector: %s", err)
	}

	// if !config.OpenTelemetryEnableMetrics {
	// 	return traceExporter, nil, err
	// }

	mtctx, mtcancel := context.WithTimeout(context.Background(), metricsConnTimeout)
	defer mtcancel()

	metricExporter, err := otlpmetrichttp.New(mtctx, meterOpts...)
	if err != nil {
		err = fmt.Errorf("Can't connect to OpenTelemetry collector: %s", err)
	}

	return traceExporter, metricExporter, err
}

func getConnectionTimeouts() (time.Duration, time.Duration, error) {
	connTimeout := 10000
	// configurators.Int(&connTimeout, "OTEL_EXPORTER_OTLP_TIMEOUT")

	tracesConnTimeout := connTimeout
	// configurators.Int(&tracesConnTimeout, "OTEL_EXPORTER_OTLP_TRACES_TIMEOUT")

	metricsConnTimeout := connTimeout
	// configurators.Int(&metricsConnTimeout, "OTEL_EXPORTER_OTLP_METRICS_TIMEOUT")

	if tracesConnTimeout <= 0 {
		return 0, 0, errors.New("Opentelemetry traces timeout should be greater than 0")
	}

	if metricsConnTimeout <= 0 {
		return 0, 0, errors.New("Opentelemetry metrics timeout should be greater than 0")
	}

	return time.Duration(tracesConnTimeout) * time.Millisecond,
		time.Duration(metricsConnTimeout) * time.Millisecond,
		nil
}

func buildTLSConfig() (*tls.Config, error) {
	// if len(config.OpenTelemetryServerCert) == 0 {
	// 	return nil, nil
	// }

	certPool := x509.NewCertPool()
	// if !certPool.AppendCertsFromPEM(prepareKeyCert(config.OpenTelemetryServerCert)) {
	// 	return nil, errors.New("Can't load OpenTelemetry server cert")
	// }

	tlsConf := tls.Config{RootCAs: certPool}

	// if len(config.OpenTelemetryClientCert) > 0 && len(config.OpenTelemetryClientKey) > 0 {
	// 	cert, err := tls.X509KeyPair(
	// 		prepareKeyCert(config.OpenTelemetryClientCert),
	// 		prepareKeyCert(config.OpenTelemetryClientKey),
	// 	)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("Can't load OpenTelemetry client cert/key pair: %s", err)
	// 	}

	// 	tlsConf.Certificates = []tls.Certificate{cert}
	// }

	return &tlsConf, nil
}

func prepareKeyCert(str string) []byte {
	return []byte(strings.ReplaceAll(str, `\n`, "\n"))
}
