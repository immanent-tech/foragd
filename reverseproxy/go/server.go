// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reverseproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	otelchimetric "github.com/riandyrn/otelchi/metric"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/otel"
)

const (
	gracefulShutdownTimeout = 30 * time.Second
)

// Start will start the server.
func Start(logger *slog.Logger) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()

	ctx = slogctx.NewCtx(ctx, logger)

	// Load the server config.
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	var err error

	// Set up OpenTelemetry.
	otelConfig, otelShutdown, err := otel.Setup(ctx)
	if err != nil {
		return fmt.Errorf("unable to set up open telemetry: %w", err)
	}

	// define base config for metric middlewares
	otelMetricConfig := otelchimetric.NewBaseConfig(config.AppName, otelchimetric.WithMeterProvider(otelConfig.Meter))

	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Set up a new chi router.
	router := chi.NewRouter()

	// Health check endpoints (for GCP).
	router.Use(middleware.Heartbeat("/health-check"))

	// Standard middleware stack.
	router.Use(
		middleware.RequestID,
		middlewares.Logger,
		middleware.Recoverer,
		middleware.StripSlashes,
		// middlewares.Etag,
		otelchi.Middleware(config.AppName,
			otelchi.WithChiRoutes(router),
			otelchi.WithTracerProvider(otelConfig.Tracer),
		),
		otelchimetric.NewRequestDurationMillis(otelMetricConfig),
		otelchimetric.NewRequestInFlight(otelMetricConfig),
		otelchimetric.NewResponseSizeBytes(otelMetricConfig))

	router.Get("/{signature}/{encodedURL}", handleReverseProxy)

	h2s := &http2.Server{}
	svr := &http.Server{
		Handler:      h2c.NewHandler(router, h2s),
		Addr:         net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadTimeout:  cfg.ReadTimeout.Duration(),
		WriteTimeout: cfg.WriteTimeout.Duration(),
		IdleTimeout:  cfg.IdleTimeout.Duration(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	err = http2.ConfigureServer(svr, h2s)
	if err != nil {
		return fmt.Errorf("unable to configure server for H2C: %w", err)
	}

	logger.Info("Starting server...",
		slog.String("address", svr.Addr),
		slog.Time("start_time", time.Now()),
	)

	// And we serve HTTP until the world ends.
	go func() {
		var err error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			logger.Info("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			err = svr.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			logger.Info("Using http.")
			err = svr.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("Could not listen.",
				slog.Any("error", err),
			)
		}
	}()

	<-ctx.Done()

	// Create shutdown context with 30-second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	// Trigger graceful shutdown
	logger.Info("Shutting down server...")
	if err := svr.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}

	logger.Info("Server shutdown gracefully")

	return nil
}
