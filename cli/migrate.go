// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

// MigrateCmd defines the `migrate` command, which performs data-store migrations for schema changes.
type MigrateCmd struct {
	Migrations []string `help:"Components to migrate."`
}

// Run contains the logic for performing the migrate command.
func (r *MigrateCmd) Run(_ *Arguments) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Load the Elastic backend
	elasticClient, err := elastic.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}

	if err = schema.Migration(ctx, elasticClient.GetAPI(), r.Migrations...); err != nil {
		return fmt.Errorf("unable to perform Elastic migration: %w", err)
	}

	return nil
}
