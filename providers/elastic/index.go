// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

func GenerateIndexName(prefix string) string {
	return strings.Join(
		[]string{prefix, config.GetEnvironment().String(), time.Now().Format("20060102150405"), "000000"},
		"-",
	)
}

func CreateIndexIfNotExists(ctx context.Context, prefix string) (bool, error) {
	if err := Connect(); err != nil {
		return false, fmt.Errorf("connect to elasticsearch: %w", err)
	}

	index := GenerateIndexName(prefix)
	// Create index.
	found, err := api.Indices.Exists(index).Do(ctx)
	if err != nil {
		return found, fmt.Errorf("could not determine %s index state: %w", index, err)
	}
	if !found {
		_, err = api.Indices.Create(index).Do(ctx)
		if err != nil {
			return found, fmt.Errorf("could not create index %s: %w", index, err)
		}
		slogctx.FromCtx(ctx).Info("New index created.",
			slog.String("name", index),
		)
	}
	slogctx.FromCtx(ctx).Info("Index already exists.",
		slog.String("name", index),
	)

	return found, nil
}

// UpdateIndexAlias performs a swap of an alias to the given index. It adds the given index to the alias, sets it as the
// write destination, then removes any existing aliased indicies so the index remains as the only aliased one.
//
// https://www.elastic.co/docs/manage-data/data-store/aliases
func UpdateIndexAlias(ctx context.Context, alias string, index string) error {
	if err := Connect(); err != nil {
		return fmt.Errorf("connect to elasticsearch: %w", err)
	}

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
