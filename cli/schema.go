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
	Update  UpdateSchemaCmd  `cmd:"update"  help:"Update schema(s)"`
	Migrate MigrateDataCmd   `cmd:"migrate" help:"Migrate data"`
	Create  CreateIndciesCmd `cmd:"create"  help:"Create indices"`
}

type UpdateSchemaCmd struct {
	schema.IndicesOptions
}

func (r *UpdateSchemaCmd) Run(opts *UpdateSchemaCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	// Perform migrations.
	err = schema.UpdateIndicesSchema(ctx, elasticClient.TypedClient, &opts.IndicesOptions)
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	return nil
}

type MigrateDataCmd struct {
	schema.IndicesOptions
}

func (r *MigrateDataCmd) Run(opts *MigrateDataCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	// Perform migrations.
	err = schema.MigrateIndices(ctx, elasticClient.TypedClient, &opts.IndicesOptions)
	if err != nil {
		return fmt.Errorf("update schemas: %w", err)
	}
	return nil
}

type CreateIndciesCmd struct {
	schema.IndicesOptions
}

func (r *CreateIndciesCmd) Run(opts *CreateIndciesCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}
	// Perform migrations.
	err = schema.CreateIndices(ctx, elasticClient.TypedClient, &opts.IndicesOptions)
	if err != nil {
		return fmt.Errorf("create indices: %w", err)
	}
	return nil
}
