// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:unused-receiver
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/internal/server"
)

// MigrateCmd: `go-feed-me migrate`.
type MigrateCmd struct {
	Migrations []string `help:"Components to migrate."`
}

func (r *MigrateCmd) Run(opts *CmdOpts) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	ctx = logging.ToContext(ctx, opts.Logger)

	// Load the config.
	if err := server.LoadConfig(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	// Load the Elastic backend
	elasticClient, err := elastic.Connect(ctx, server.Environment())
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}

	if err = schema.Migration(ctx, elasticClient, r.Migrations...); err != nil {
		return fmt.Errorf("unable to perform Elastic migration: %w", err)
	}

	return nil
}
