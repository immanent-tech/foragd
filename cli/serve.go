// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/config"
	"github.com/immanent-tech/go-feed-me/server"
)

const (
	// ServerReadTimeout is the default read timeout for the server.
	ServerReadTimeout = 5 * time.Second
	// ServerWriteTimeout is the default write timeout for the server.
	ServerWriteTimeout = 10 * time.Second
)

// ServeCmd defines the `server` command for running the server.
type ServeCmd struct{}

// Run performs setup and execution for the server command.
func (r *ServeCmd) Run(opts *Arguments) error {
	// Creating a waiting group that waits until the graceful shutdown procedure is done
	var wg sync.WaitGroup

	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)

	// Set up a new server interface.
	svr, err := server.NewServer(ctx, opts.StaticContent)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}

	wg.Add(1)
	// Listen for shutdown events and process them.
	go func() {
		wg.Done()

		stop := make(chan os.Signal, 1)

		signal.Notify(stop, os.Interrupt)
		<-stop

		err = svr.Shutdown(context.Background())
		// Can't do much here except for logging any errors
		if err != nil {
			slog.Error("Error occurred when trying to shut down server.",
				slog.Any("error", err),
			)
		}
	}()

	slogctx.FromCtx(ctx).Info("Starting server...",
		slog.Int("port", server.ServerConfig.Port),
		slog.String("environment", config.Environment()))

	// And we serve HTTP until the world ends.
	err = svr.ListenAndServeTLS("localhost.crt", "localhost.key")
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		slogctx.FromCtx(ctx).Info("commencing server shutdown...")
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}
