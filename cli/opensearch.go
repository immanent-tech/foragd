/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/immanent-tech/go-base/validation"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/indices"
	"github.com/immanent-tech/foragd/providers/opensearch/ml"
	"github.com/immanent-tech/foragd/schema"
)

// OpenSearchCmd contains subcommands for interacting with OpenSearch.
type OpenSearchCmd struct {
	CreateIndices   OSCreateIndicesCmd  `cmd:"" help:"create indices"`
	UpdateMappings  OSUpdateMappingsCmd `cmd:"" help:"update mappings"`
	Configure       OSConfigureCmd      `cmd:"" help:"configure opensearch"`
	MachineLearning OSMLCmd             `cmd:"" help:"machine learning commands"`
}

// OSCreateIndicesCmd creates the specified indices, their index and component templates and assigns read/write aliases
// for the indices.
type OSCreateIndicesCmd struct {
	Indices []opensearch.Index `help:"list of indices" validate:"omitempty,unique"`
}

func (c *OSCreateIndicesCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	if len(c.Indices) == 0 {
		c.Indices = schema.AllIndices
	}
	if slices.ContainsFunc(c.Indices, func(e opensearch.Index) bool {
		return e.String() == "all"
	}) {
		c.Indices = schema.AllIndices
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		return fmt.Errorf("get opensearch client: %w", err)
	}

	// Create the common component template.
	if err := indices.CreateComponentTemplate(ctx, osc.Cluster, schema.CommonComponentTemplate); err != nil {
		return fmt.Errorf("create common component template: %w", err)
	}

	for index := range slices.Values(c.Indices) {
		// Generate index config.
		config, err := schema.NewIndexConfig(osc, index)
		if err != nil {
			return fmt.Errorf("generate index config %s: %w", index.String(), err)
		}

		if err := config.InitIndex(ctx, osc); err != nil {
			return fmt.Errorf("init index %s: %w", index, err)
		}
	}

	return nil
}

// OSUpdateMappingsCmd updates the specified indices component and index templates and existing index mappings.
type OSUpdateMappingsCmd struct {
	Indices []opensearch.Index `help:"list of indices" validate:"omitempty,unique"`
}

func (c *OSUpdateMappingsCmd) Run() error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	if len(c.Indices) == 0 {
		c.Indices = schema.AllIndices
	}
	if slices.ContainsFunc(c.Indices, func(e opensearch.Index) bool {
		return e.String() == "all"
	}) {
		c.Indices = schema.AllIndices
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		return fmt.Errorf("get opensearch client: %w", err)
	}

	// Create the common component template.
	if err := indices.CreateComponentTemplate(ctx, osc.Cluster, schema.CommonComponentTemplate); err != nil {
		return fmt.Errorf("create common component template: %w", err)
	}

	for index := range slices.Values(c.Indices) {
		// Generate index config.
		config, err := schema.NewIndexConfig(osc, index)
		if err != nil {
			return fmt.Errorf("generate index config %s: %w", index.String(), err)
		}

		if err := config.UpdateIndex(ctx, osc); err != nil {
			return fmt.Errorf("update index %s: %w", index, err)
		}
	}

	return nil
}

// OSConfigureCmd performs one-off configuration of OpenSearch, such as applying persistent cluster settings.
type OSConfigureCmd struct{}

func (c *OSConfigureCmd) Run() error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		return fmt.Errorf("get opensearch client: %w", err)
	}

	if err := osc.ApplyClusterSettings(ctx); err != nil {
		return fmt.Errorf("configure opensearch: %w", err)
	}

	slogctx.Info(ctx, "Cluster settings applied.")

	return nil
}

// OSMLCmd performs one-off configuration of the ML plugin.
type OSMLCmd struct {
	Init OSMLInitCmd `cmd:"" help:"init ML"`
}

type OSMLInitCmd struct{}

func (c *OSMLInitCmd) Run() error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	if err := validation.Validate.Struct(c); err != nil {
		return fmt.Errorf("validate options: %w", err)
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		return fmt.Errorf("get opensearch client: %w", err)
	}

	mlc, err := ml.NewClient(osc)
	if err != nil {
		return fmt.Errorf("create ml client: %w", err)
	}

	connectorID, modelID, err := mlc.Configure(ctx)
	if err != nil {
		return fmt.Errorf("configure connector: %w", err)
	}

	slogctx.Info(ctx, "ML Configured", slog.String("connector_id", connectorID), slog.String("model_id", modelID))

	return nil
}
