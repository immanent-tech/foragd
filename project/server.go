// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	gowebly "github.com/gowebly/helpers"

	"github.com/joshuar/go-feed-me/platform/feeds"
	"github.com/joshuar/go-feed-me/platform/scheduler"
	"github.com/joshuar/go-feed-me/server"
)

const (
	ServerReadTimeout  = 5 * time.Second
	ServerWriteTimeout = 10 * time.Second
)

//go:embed all:static
var static embed.FS

func runServer() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	// Set up a new server interface.
	svr, err := server.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("could not start: %w", err)
	}

	scheduler.Start(ctx, svr.Logger.Handler())
	feeds.NewGetFeedsWorker(ctx, svr.StoreAPI(), svr.DataAPI())

	// Set up a new chi router.
	router := chi.NewRouter()
	router.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(static)))

	// Get an `http.Handler` that we can use from the router and server.
	handler := server.GenerateHandler(svr, router)

	s := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf(":%d", svr.GetPort()),
		ReadTimeout:  ServerReadTimeout,
		WriteTimeout: ServerWriteTimeout,
	}

	svr.Logger.Info("Starting server...",
		slog.Int("port", svr.GetPort()),
		slog.String("environment", svr.GetEnvironment()))

	// And we serve HTTP until the world ends.
	return s.ListenAndServe()
}
