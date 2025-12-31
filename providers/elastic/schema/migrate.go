// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/ingest/putpipeline"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
)

// Migrate performs all requested schema migrations.
func Migrate(ctx context.Context, api *elasticsearch.TypedClient, opts *Options) error {
	// If no migrations are specified, perform migrations for all items.
	if slices.Contains(opts.Indices, "all") {
		opts.Indices = []string{"users", "feeds", "items", "favorites", "scheduler", "sessions", "subscriptions"}
	}

	for index := range slices.Values(opts.Indices) {
		var err error
		switch index {
		case "users":
			err = migrateIndexData(ctx, api, UsersSchemaPrefix, nil) // ingest.NewIngestPipeline(
			// 	ingest.WithProcessor(types.ProcessorContainer{
			// 		Rename: &types.RenameProcessor{
			// 			Field:       "settings.max_history",
			// 			TargetField: "metadata.max_history",
			// 		},
			// 	}),
			// 	ingest.WithProcessor(types.ProcessorContainer{
			// 		Rename: &types.RenameProcessor{
			// 			Field:       "settings.updates_frequency",
			// 			TargetField: "metadata.updates_frequency",
			// 		},
			// 	}),
			// ),
		case "scheduler":
			err = migrateIndexData(ctx, api, schedulerIndexPrefix, nil) // ingest.NewIngestPipeline(
		}
		if err != nil {
			return fmt.Errorf("could not migrate users: %w", err)
		}
	}
	return nil
}

func migrateIndexData(
	ctx context.Context,
	api *elasticsearch.TypedClient,
	prefix string,
	pipeline *putpipeline.Request,
) error {
	index := strings.Join([]string{prefix, config.Version, time.Now().Format("20060102150405")}, "-")
	writeAlias := prefix + IndexWriteSuffix
	readAlias := prefix + IndexReadSuffix

	// If a pipeline is specified, create it.
	var pipelineName string
	if pipeline != nil {
		pipelineName = "pipeline-" + index
		if _, err := api.Ingest.PutPipeline(pipelineName).Request(pipeline).Do(ctx); err != nil {
			return fmt.Errorf("migrate index %s: put pipeline: %w", index, err)
		}
	}

	// Create index.
	found, err := api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not determine %s index state: %w", index, err)
	}
	if !found {
		_, err = api.Indices.Create(index).Do(ctx)
		if err != nil {
			return fmt.Errorf("could not create index %s: %w", index, err)
		}
	}
	slogctx.FromCtx(ctx).Info("New index created",
		slog.String("name", index),
	)
	// Update the write alias.
	err = updateAlias(ctx, api, writeAlias, index)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	// Reindex if requested.
	found, err = api.Indices.Exists(readAlias).Do(ctx)
	if err != nil || !found {
		return fmt.Errorf("could not determine %s index state: %w", readAlias, err)
	}
	reindexResp, err := reindex.NewReindexOperation(api, reindex.NewSource(readAlias), reindex.NewDest(index, pipelineName)).
		WaitForCompletion(true).
		Do(ctx)
	const statusCodeErrLevel = 500
	switch {
	case err != nil:
		if getStatusCode(err) >= statusCodeErrLevel {
			return fmt.Errorf("could not reindex: %w", err)
		}
		slogctx.FromCtx(ctx).Info("Reindex completed with warnings.",
			slog.String("src", readAlias),
			slog.String("dest", index),
			slog.Int64("took", *reindexResp.Took),
			slog.Any("warnings", err),
		)
	default:
		slogctx.FromCtx(ctx).Info("Reindex completed.",
			slog.String("src", readAlias),
			slog.String("dest", index),
			slog.Int64("took", *reindexResp.Took),
		)
	}
	// Update the read alias.
	err = updateAlias(ctx, api, readAlias, index)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}

// updateAlias performs a swap of an alias to the given index. It adds the given index to the alias, sets it as the
// write destination, then removes any existing aliased indicies so the index remains as the only aliased one.
//
// https://www.elastic.co/docs/manage-data/data-store/aliases
func updateAlias(ctx context.Context, api *elasticsearch.TypedClient, alias string, index string) error {
	aliasesResp, err := api.Indices.GetAlias().Index(alias).Do(ctx)
	if err != nil {
		if getStatusCode(err) != http.StatusNotFound {
			return fmt.Errorf("could not retrieve indices associated with alias %s: %w", alias, err)
		}
	}
	// Remove existing index marked as write index from alias.
	for aliasedIndex, aliases := range aliasesResp {
		if _, found := aliases.Aliases[alias]; found {
			_, err = api.Indices.DeleteAlias(aliasedIndex, alias).Do(ctx)
			if err != nil {
				return fmt.Errorf("unable to remove index %s from alias %s: %w", aliasedIndex, alias, err)
			}
			slogctx.FromCtx(ctx).Info("Removed index for alias.",
				slog.String("alias", alias),
				slog.String("old_index", aliasedIndex),
			)
		}
	}

	var writeIndex bool
	if strings.HasSuffix(alias, "rw") {
		// Set as write index if alias name ends in "rw".
		writeIndex = true
	}
	// Update the alias.
	_, err = api.Indices.PutAlias(index, alias).IsWriteIndex(writeIndex).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not update alias %s to add index %s: %w", alias, index, err)
	}
	slogctx.FromCtx(ctx).Info("Index alias updated.",
		slog.String("alias", alias),
		slog.String("index", index),
		slog.Bool("is_write_index", writeIndex),
	)
	return nil
}
