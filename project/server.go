// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	gowebly "github.com/gowebly/helpers"

	"github.com/joshuar/go-feed-me/internal/scheduler"
	"github.com/joshuar/go-feed-me/internal/server"
)

const (
	ServerReadTimeout  = 5 * time.Second
	ServerWriteTimeout = 10 * time.Second
)

var ErrStartServerFailed = errors.New("could not start server")

//go:embed all:static
var static embed.FS

func runServer() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	// Set up a new server interface.
	svr, err := server.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrStartServerFailed, err)
	}

	// Start a task worker for this server.
	if err := scheduler.NewTaskWorker(ctx, svr.DataAPI(), svr.StoreAPI()); err != nil {
		return fmt.Errorf("%w: %w", ErrStartServerFailed, err)
	}

	// Start the scheduler.
	if err := scheduler.NewTaskScheduler(ctx, svr.DataAPI()); err != nil {
		return fmt.Errorf("%w: %w", ErrStartServerFailed, err)
	}

	// Set up a new chi router.
	router := chi.NewRouter()
	router.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(static)))

	// Get an `http.Handler` that we can use from the router and server.
	handler := server.GenerateHandler(svr, router)

	serverObj := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf(":%d", svr.Port()),
		ReadTimeout:  ServerReadTimeout,
		WriteTimeout: ServerWriteTimeout,
	}

	svr.Logger.Info("Starting server...",
		slog.Int("port", svr.Port()),
		slog.String("environment", svr.Environment()))

	// And we serve HTTP until the world ends.
	return serverObj.ListenAndServe()
}
