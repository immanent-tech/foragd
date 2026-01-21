// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
)

type SchemaCmd struct {
	Update  UpdateCmd  `cmd:"update"  help:"Update schema(s)"`
	Migrate MigrateCmd `cmd:"migrate" help:"Migrate schema(s)"`
}

type UpdateCmd struct {
	schema.Options
}

func (r *UpdateCmd) Run(opts *UpdateCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	// Perform migrations.
	err = schema.CreateSchemas(ctx, elasticClient.TypedClient, &opts.Options)
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	return nil
}

type MigrateCmd struct {
	schema.Options
}

func (r *MigrateCmd) Run(opts *MigrateCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	// Perform migrations.
	err = schema.Migrate(ctx, elasticClient.TypedClient, &opts.Options)
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	return nil
}
