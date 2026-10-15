/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
)

const (
	// readinessDrainDelay is the time to keep serving (while reporting not-ready) so the load balancer can stop routing
	// to us. Set to 0 for local development. Consider making this configurable.
	readinessDrainDelay = 5 * time.Second
	// gracefulShutdownTimeout is the time allowed for in-flight requests to finish and dependencies to flush.
	gracefulShutdownTimeout = 30 * time.Second
	// otelShutdownTimeout is the time allowed for OpenTelemetry to flush its final batch.
	otelShutdownTimeout = 5 * time.Second
)

var (
	// ready reports whether the server should receive new traffic. Flipped to false on shutdown.
	ready atomic.Bool
	// shuttingDown is closed when the server begins shutting down. Long-lived handlers (SSE, long-poll) should select
	// on ShuttingDown() so they don't stall svr.Shutdown.
	shuttingDown = make(chan struct{})
)

// ShuttingDown returns a channel that is closed when the server starts shutting down.
func ShuttingDown() <-chan struct{} { return shuttingDown }

// readiness is a middleware that answers /readyz before any other middleware runs.
func readiness(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			if !ready.Load() {
				http.Error(w, "shutting down", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Start will start the server.
func Start() error {
	logger := logging.New()
	startTime := time.Now()

	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()

	ctx = slogctx.NewCtx(ctx, logger)

	//  Load the app config and server config first: other components may depend on them.
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	services, err := loadServices(ctx, appCfg)
	if err != nil {
		return fmt.Errorf("load services: %w", err)
	}

	// Flush OpenTelemetry last, after everything else has stopped producing telemetry.
	// Runs on every exit path, and the error is propagated via the named return.
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), otelShutdownTimeout)
		defer cancel()
		err = errors.Join(err, services.otelShutdown(c))
	}()

	// Requests must NOT inherit the signal context, otherwise SIGTERM cancels every in-flight request
	// immediately and svr.Shutdown has nothing left to drain. WithoutCancel keeps the logger/values.
	baseCtx := context.WithoutCancel(ctx)

	svr := &http.Server{
		Protocols:         new(http.Protocols),
		Handler:           buildRouter(services, appCfg),
		Addr:              net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadHeaderTimeout: cfg.ReadTimeout.Duration,
		ReadTimeout:       cfg.ReadTimeout.Duration,
		WriteTimeout:      cfg.WriteTimeout.Duration,
		IdleTimeout:       cfg.IdleTimeout.Duration,
		BaseContext: func(_ net.Listener) context.Context {
			return baseCtx
		},
	}
	svr.Protocols.SetUnencryptedHTTP2(true) // Enable H2C (HTTP/2 cleartext)
	svr.Protocols.SetHTTP1(true)            // Enable HTTP/1.1
	svr.Protocols.SetHTTP2(false)           // Explicitly disable encrypted HTTP/2 (HTTPS)
	// Lets long-lived handlers (SSE, long-poll) exit via ShuttingDown() instead of stalling Shutdown.
	svr.RegisterOnShutdown(func() { close(shuttingDown) })

	// Bind synchronously so bind errors (port in use, etc.) are returned immediately and the
	// "Server started" message is accurate.
	ln, err := net.Listen("tcp", svr.Addr)
	if err != nil {
		// Dependencies are already initialised, so release them on this path too.
		return errors.Join(fmt.Errorf("listen on %s: %w", svr.Addr, err), shutdown(svr, logger))
	}

	// And we serve HTTP until the world ends.
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		var serveErr error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			logger.Debug("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			serveErr = svr.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
		} else {
			logger.Debug("Using http.")
			serveErr = svr.Serve(ln)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	ready.Store(true)
	logger.Info("Server started...",
		slog.String("address", svr.Addr),
		slog.String("version", appCfg.GetAppVersion()),
		slog.Time("start_time", time.Now().UTC()),
		slog.Duration("startup_took", time.Since(startTime)),
	)

	var serveErr error
	select {
	case serveErr = <-errCh:
		if serveErr != nil {
			serveErr = fmt.Errorf("server failed: %w", serveErr)
		}
	case <-ctx.Done():
		slog.Info("Shutting down server")
	}

	// Stop catching signals so a second Ctrl-C force-kills the process.
	cancelFunc()

	// Tell the load balancer to stop routing to us, and keep serving while it catches up.
	ready.Store(false)
	if serveErr == nil && readinessDrainDelay > 0 {
		time.Sleep(readinessDrainDelay)
	}

	// Cleanup always runs, whether we got here from a signal or a serve failure.
	return errors.Join(serveErr, shutdown(svr, logger))
}

// shutdown stops the server first (so no new requests arrive and in-flight ones finish), and only then
// tears down dependencies, in the reverse order they are used. OpenTelemetry is flushed last by the
// deferred call in Start.
func shutdown(svr *http.Server, logger *slog.Logger) error {
	// The shutdown context must not derive from the (already cancelled) signal context.
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		gracefulShutdownTimeout,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, logger)
	defer cancel()

	var errs []error

	// 1. Stop accepting connections and wait for in-flight requests to complete.
	if err := svr.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("server graceful shutdown: %w", err))
		// Timed out: force-close remaining connections so the process can exit.
		if closeErr := svr.Close(); closeErr != nil {
			errs = append(errs, fmt.Errorf("server close: %w", closeErr))
		}
	}

	// 2. TODO: stop background work (import jobs, Resend processing) and wait for it to finish,
	// e.g. importSvc.Shutdown(shutdownCtx) backed by a sync.WaitGroup.

	// 3. Flush and close the bulk indexer (if initialized).
	if err := bulk.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Bulk indexer failed to shutdown gracefully.",
			slog.Any("error", err),
		)
		errs = append(errs, fmt.Errorf("bulk indexer shutdown: %w", err))
	}
	// 4. Then close any Elasticsearch connection (if initialized).
	if err := elastic.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Elasticsearch failed to shutdown gracefully.",
			slog.Any("error", err),
		)
		errs = append(errs, fmt.Errorf("elasticsearch shutdown: %w", err))
	}

	// 5. TODO: close caches / session store if they expose a Close method.

	// 6. Close error client.
	gerror.CloseClient()

	slogctx.FromCtx(shutdownCtx).Info("Server stopped",
		slog.Time("stop_time", time.Now().UTC()),
	)

	return errors.Join(errs...)
}
