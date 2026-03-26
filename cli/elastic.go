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

// ElasticCmd contains commands for manipulating Elasticsearch.
type ElasticCmd struct {
	Indices IndicesCmd `cmd:"indices" help:"Perform operations on indices."`
	ILM     ILMCmd     `cmd:"ilm"     help:"Perform ILM operations."`
}

// IndicesCmd contains commands for manipulating Elasticsearch indices.
type IndicesCmd struct {
	Update  UpdateIndexSchemaCmd `cmd:"update"  help:"Update schema(s)"`
	Migrate MigrateIndexCmd      `cmd:"migrate" help:"Migrate data"`
	Create  CreateIndexCmd       `cmd:"create"  help:"Create indices"`
}

type UpdateIndexSchemaCmd struct {
	schema.IndicesOptions
}

func (r *UpdateIndexSchemaCmd) Run(opts *UpdateIndexSchemaCmd) error {
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

type MigrateIndexCmd struct {
	schema.IndicesOptions
}

func (r *MigrateIndexCmd) Run(opts *MigrateIndexCmd) error {
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

type CreateIndexCmd struct {
	schema.IndicesOptions
}

func (r *CreateIndexCmd) Run(opts *CreateIndexCmd) error {
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

type ILMCmd struct {
	Update UpdateILMPoliciesCmd `cmd:"update" help:"Update ILM policies."`
}

type UpdateILMPoliciesCmd struct {
	schema.ILMOptions
}

func (r *UpdateILMPoliciesCmd) Run(opts *UpdateILMPoliciesCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.NewConnection()
	if err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}
	// Perform migrations.
	err = schema.UpdateILMPolicies(ctx, elasticClient.TypedClient, &opts.ILMOptions)
	if err != nil {
		return fmt.Errorf("create indices: %w", err)
	}
	return nil

}
