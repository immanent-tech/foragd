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
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/indices"
	"github.com/immanent-tech/foragd/providers/opensearch/ingest"
	"github.com/immanent-tech/foragd/providers/opensearch/ml"
	"github.com/immanent-tech/foragd/schema"
)

// OpenSearchCmd contains subcommands for interacting with OpenSearch.
type OpenSearchCmd struct {
	CreateIndices   OSCreateIndicesCmd `cmd:"" help:"create indices"`
	Configure       OSConfigureCmd     `cmd:"" help:"configure opensearch"`
	MachineLearning OSMLCmd            `cmd:"" help:"machine learning commands"`
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

	mlc, err := ml.NewClient(osc)
	if err != nil {
		return fmt.Errorf("get opensearch ml client: %w", err)
	}

	// Create the common component template.
	if err := indices.CreateComponentTemplate(ctx, osc.Cluster, schema.CommonComponentTemplate); err != nil {
		return fmt.Errorf("create common component template: %w", err)
	}

	for index := range slices.Values(c.Indices) {
		var (
			indexName         string
			indexTemplate     opensearchapi.IndicesPutIndexTemplateReq
			componentTemplate opensearchapi.ClusterPutComponentTemplateReq
			readAlias         string
			writeAlias        string
		)
		switch index {
		case schema.FeedsIndex:
			indexName = osc.IndexCanonicalName(index)
			indexTemplate = schema.FeedsIndexTemplate
			componentTemplate = schema.FeedsComponentTemplate
			readAlias = osc.IndexReadAlias(index)
			writeAlias = osc.IndexWriteAlias(index)
		case schema.ItemsIndex:
			indexName = osc.IndexCanonicalName(index)
			indexTemplate = schema.ItemsIndexTemplate
			componentTemplate = schema.ItemsComponentTemplate
			readAlias = osc.IndexReadAlias(index)
			writeAlias = osc.IndexWriteAlias(index)
			// Additional steps for items index.
			if err := ingest.NewIngestPipeline(
				ctx,
				&osc.Ingest,
				schema.ItemsIngestPipeline(mlc.GetModelID()),
			); err != nil {
				return fmt.Errorf("configure items ingest pipeline: %w", err)
			}
		case schema.SessionsIndex:
			indexName = osc.IndexCanonicalName(index)
			indexTemplate = schema.SessionsIndexTemplate
			componentTemplate = schema.SessionsComponentTemplate
			readAlias = osc.IndexReadAlias(index)
			writeAlias = osc.IndexWriteAlias(index)
		case schema.SchedulerIndex:
			indexName = osc.IndexCanonicalName(index)
			indexTemplate = schema.SchedulerIndexTemplate
			componentTemplate = schema.SchedulerComponentTemplate
			readAlias = osc.IndexReadAlias(index)
			writeAlias = osc.IndexWriteAlias(index)
		default:
			return fmt.Errorf("unknown index: %s", index)
		}

		// Create component template.
		if err := indices.CreateComponentTemplate(ctx, osc.Cluster, componentTemplate); err != nil {
			return fmt.Errorf("create %s component template: %w", index, err)
		}
		slogctx.Info(ctx, "Created component template",
			slog.String("name", componentTemplate.Name),
		)

		// Create index template.
		if err := indices.CreateIndexTemplate(ctx, osc.Index, indexTemplate); err != nil {
			return fmt.Errorf("create %s index template: %w", index, err)
		}
		slogctx.Info(ctx, "Created index template",
			slog.String("name", indexTemplate.Name),
		)

		// Create index.
		if err := indices.CreateIndexIfNotExists(ctx, osc.Index, indexName); err != nil {
			return fmt.Errorf("create index %s: %w", osc.IndexCanonicalName(index), err)
		}
		slogctx.Info(ctx, "Created index",
			slog.String("name", indexName),
		)

		// Create aliases.
		if err := indices.UpdateIndexAlias(ctx, osc.Index, writeAlias, indexName, true); err != nil {
			return fmt.Errorf("create write alias %s->%s: %w", writeAlias, indexName, err)
		}
		slogctx.Info(ctx, "Created write alias",
			slog.String("alias", writeAlias),
			slog.String("target", indexName),
		)
		if err := indices.UpdateIndexAlias(ctx, osc.Index, readAlias, indexName, false); err != nil {
			return fmt.Errorf("create read alias %s->%s: %w", writeAlias, indexName, err)
		}
		slogctx.Info(ctx, "Created read alias",
			slog.String("alias", readAlias),
			slog.String("target", indexName),
		)
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
