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

type SchemaCmd struct {
	Create CreateCmd `cmd:"create" help:"Create schemas"`
}

type CreateCmd struct {
	schema.Options
}

func (r *CreateCmd) Run(opts *CreateCmd) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	// Load the Elastic backend
	elasticClient, err := elastic.Connect(ctx)
	if err != nil {
		return fmt.Errorf("create schemas: %w", err)
	}
	// Perform migrations.
	err = schema.CreateSchemas(ctx, elasticClient.GetAPI(), &opts.Options)
	if err != nil {
		return fmt.Errorf("create schemas: %w", err)
	}
	return nil
}
