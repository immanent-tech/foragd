// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/server"
)

// ServeCmd defines the `server` command for running the server.
type ServeCmd struct{}

// Run performs setup and execution for the server command.
func (r *ServeCmd) Run(opts *Arguments) error {
	// Creating a waiting group that waits until the graceful shutdown procedure is done

	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)

	// Set up a new server interface.
	svr, err := server.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}

	err = svr.Start(ctx)
	if err != nil {
		return fmt.Errorf("unable to run server: %w", err)
	}

	return nil
}
