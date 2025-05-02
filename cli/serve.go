// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	gowebly "github.com/gowebly/helpers"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/server"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/server/middlewares"
)

// ErrServerCmd indicates an error occurred when running the server command.
var ErrServerCmd = errors.New("error running server command")

const (
	// ServerReadTimeout is the default read timeout for the server.
	ServerReadTimeout = 5 * time.Second
	// ServerWriteTimeout is the default write timeout for the server.
	ServerWriteTimeout = 10 * time.Second
)

// ServeCmd defines the `server` command for running the server.
type ServeCmd struct{}

// Run performs setup and execution for the server command.
//
//nolint:funlen
func (r *ServeCmd) Run(opts *Arguments) error {
	// Creating a waiting group that waits until the graceful shutdown procedure is done
	var wg sync.WaitGroup

	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)

	// Set up a new server interface.
	svr, err := server.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServerCmd, err)
	}

	// Set up a new chi router.
	router := chi.NewRouter()
	router.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(opts.StaticContent)))

	// Get an `http.Handler` that we can use from the router and server.
	// handler := server.GenerateHandler(svr, router)
	handler := server.HandlerWithOptions(svr, server.ChiServerOptions{
		BaseRouter: router,
		Middlewares: []server.MiddlewareFunc{
			slogchi.NewWithConfig(slog.Default(), slogchi.Config{WithRequestID: true}),
			middleware.Recoverer,
			middleware.RequestID,
			middleware.RealIP,
			// middlewares.CORS(config.Environment()),
			// middlewares.CSP(server.ServerConfig.CSP),
			middlewares.ElasticMiddleware(),
			handlers.RequireUserAuth(svr.DataAPI(), svr.AuthAPI()),
			svr.SessionAPI().LoadAndSave,
		},
	})
	serverObj := &http.Server{
		Handler:           handler,
		Addr:              fmt.Sprintf(":%d", server.Port()),
		ReadHeaderTimeout: ServerReadTimeout,
	}

	wg.Add(1)
	// Listen for shutdown events and process them.
	go func() {
		wg.Done()

		stop := make(chan os.Signal, 1)

		signal.Notify(stop, os.Interrupt)
		<-stop

		err = serverObj.Shutdown(context.Background())
		// can't do much here except for logging any errors
		if err != nil {
			log.Printf("error during shutdown: %v\n", err)
		}
	}()

	// wg.Add(1)
	// // Start the scheduler.
	// go func() {
	// 	defer wg.Done()
	// 	if err := scheduler.Run(ctx); err != nil {
	// 		svr.Log.Error("Error running scheduler.",
	// 			slog.Any("error", err))
	// 		cancelFunc()
	// 	}
	// }()

	svr.Log.Info("Starting server...",
		slog.Int("port", server.Port()),
		slog.String("environment", config.Environment()))

	// And we serve HTTP until the world ends.
	err = serverObj.ListenAndServeTLS("localhost.crt", "localhost.key")
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		svr.Log.Info("commencing server shutdown...")
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}
