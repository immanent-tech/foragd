// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

// MigrateCmd defines the `migrate` command, which performs data-store migrations for schema changes.
type SchemaCmd struct {
	Migrate MigrateCmd `cmd:"migrate" help:"Migrate schemas"`
}

type MigrateCmd struct {
	schema.SchemaOpts
}

func (r *MigrateCmd) Run(opts *MigrateCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	env := os.Getenv("FORAGD_ENVIRONMENT")
	elasticClient, err := elastic.Connect(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}
	// Perform migrations.
	err = schema.Migration(ctx, elasticClient.GetAPI(), &opts.SchemaOpts)
	if err != nil {
		return fmt.Errorf("unable to perform Elastic migration: %w", err)
	}
	return nil
}
