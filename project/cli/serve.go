// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:unused-receiver
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
	gowebly "github.com/gowebly/helpers"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server"
)

const (
	ServerReadTimeout  = 5 * time.Second
	ServerWriteTimeout = 10 * time.Second
)

var ErrStartServerFailed = errors.New("could not start server")

// ServeCmd: `go-feed-me serve`.
type ServeCmd struct{}

func (r *ServeCmd) Run(opts *CmdOpts) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Creating a waiting group that waits until the graceful shutdown procedure is done
	var wg sync.WaitGroup

	wg.Add(1)

	ctx = logging.ToContext(ctx, opts.Logger)

	// Set up a new server interface.
	svr, err := server.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrStartServerFailed, err)
	}

	// Set up a new chi router.
	router := chi.NewRouter()
	router.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(opts.StaticContent)))

	// Get an `http.Handler` that we can use from the router and server.
	handler := server.GenerateHandler(svr, router)

	serverObj := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf(":%d", server.Port()),
		ReadTimeout:  ServerReadTimeout,
		WriteTimeout: ServerWriteTimeout,
	}

	// This goroutine is running in parallels to the main one
	go func() {
		// creating a channel to listen for signals, like SIGINT
		stop := make(chan os.Signal, 1)
		// subscribing to interruption signals
		signal.Notify(stop, os.Interrupt)
		// this blocks until the signal is received
		<-stop
		// initiating the shutdown
		err = serverObj.Shutdown(context.Background())
		// can't do much here except for logging any errors
		if err != nil {
			log.Printf("error during shutdown: %v\n", err)
		}
		// notifying the main goroutine that we are done
		wg.Done()
	}()

	svr.Logger.Info("Starting server...",
		slog.Int("port", server.Port()),
		slog.String("environment", server.Environment()))

	// And we serve HTTP until the world ends.
	err = serverObj.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		svr.Logger.Info("commencing server shutdown...")
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
	}

	return nil
}
