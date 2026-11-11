/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package indices

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
)

// CreateIndexTemplate creates the given index template. If it already exists, it will be overridden.
func CreateIndexTemplate(
	ctx context.Context,
	client opensearchapi.IndicesClient,
	req opensearchapi.IndicesPutIndexTemplateReq,
) error {
	if req.Header == nil {
		req.Header = opensearch.NewRequestHeaders(ctx)
	}
	if resp, err := client.PutIndexTemplate(ctx, req); err != nil {
		return opensearch.HandleError("put index template", resp, err)
	}
	return nil
}

// CreateComponentTemplate creates the given component template. If it already exists, it will be overridden.
func CreateComponentTemplate(ctx context.Context, client opensearchapi.ClusterClient,
	req opensearchapi.ClusterPutComponentTemplateReq) error {
	if req.Header == nil {
		req.Header = opensearch.NewRequestHeaders(ctx)
	}
	if resp, err := client.PutComponentTemplate(ctx, req); err != nil {
		return opensearch.HandleError("put component template", resp, err)
	}
	return nil
}

// UpdateIndexAlias will update the given index alias to point to the given index. If the alias exists, it is deleted
// and recreated. Otherwise, it is created.
func UpdateIndexAlias(
	ctx context.Context,
	client opensearchapi.IndicesClient,
	alias, index string,
	asWriteIndex bool,
) error {
	// Get all existing aliases.
	getAliasResp, err := client.GetAlias(
		ctx,
		&opensearchapi.IndicesGetAliasReq{Header: opensearch.NewRequestHeaders(ctx)},
	)
	if err != nil {
		return opensearch.HandleError("get alias", getAliasResp, err)
	}
	// If the alias is already in use, delete it.
	for aliasedIndex, aliases := range getAliasResp.Entries {
		if _, found := aliases.Aliases[alias]; found {
			deleteAliasResp, err := client.DeleteAlias(ctx, &opensearchapi.IndicesDeleteAliasReq{
				Name:    []string{alias},
				Indices: []string{aliasedIndex},
			})
			if err != nil {
				return opensearch.HandleError("delete alias", deleteAliasResp, err)
			}
		}
	}

	// (Re)Create the index alias.
	createAliasResp, err := client.PutAlias(ctx, opensearchapi.IndicesPutAliasReq{
		Name: alias,
		Body: &opensearchapi.IndicesPutAliasBody{
			Index:        &index,
			IsWriteIndex: &asWriteIndex,
		},
	})
	if err != nil {
		return opensearch.HandleError("create alias", createAliasResp, err)
	}

	return nil
}

// AliasExists returns a boolean indicating whether the given alias exists and points to the given index. A non-nil
// error is returned if this cannot be determined.
func AliasExists(ctx context.Context, client opensearchapi.IndicesClient, alias string) (bool, error) {
	// Get the existing alias details.
	getAliasResp, err := client.GetAlias(
		ctx,
		&opensearchapi.IndicesGetAliasReq{Header: opensearch.NewRequestHeaders(ctx)},
	)
	if err != nil {
		return false, opensearch.HandleError("get alias", getAliasResp, err)
	}

	for _, aliases := range getAliasResp.Entries {
		if _, found := aliases.Aliases[alias]; found {
			return true, nil
		}
	}
	return false, nil
}

// CreateIndexIfNotExists will create the given index if it does not exist. If the index exists, the request will be a
// no-op.
func CreateIndexIfNotExists(ctx context.Context, client opensearchapi.IndicesClient, index string) error {
	exists, err := IndexExists(ctx, client, index)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, err)
	}
	if exists {
		return nil
	}
	createIndexResp, err := client.Create(ctx, opensearchapi.IndicesCreateReq{
		Header: opensearch.NewRequestHeaders(ctx),
		Index:  index,
	})
	if err != nil {
		return opensearch.HandleError("create index", createIndexResp, err)
	}

	return nil
}

// DeleteIndex removes the index with the given name.
func DeleteIndex(ctx context.Context, client opensearchapi.IndicesClient, index string) error {
	exists, err := IndexExists(ctx, client, index)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, err)
	}
	if !exists {
		return nil
	}
	deleteIndexResp, err := client.Delete(ctx, &opensearchapi.IndicesDeleteReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
	})
	if err != nil {
		return opensearch.HandleError("delete index", deleteIndexResp, err)
	}
	return nil
}

// IndexExists returns a boolean indicating whether an index with the given name exists. A non-nill error is returned if
// there was a problem determining existence.
func IndexExists(ctx context.Context, client opensearchapi.IndicesClient, index string) (bool, error) {
	getIndexResp, err := client.Get(ctx, &opensearchapi.IndicesGetReq{
		Indices: []string{index},
		Header:  opensearch.NewRequestHeaders(ctx),
	})
	if err != nil {
		if getIndexResp.Inspect().Response != nil {
			if getIndexResp.Inspect().Response.StatusCode != http.StatusNotFound {
				return false, opensearch.GenerateAPIError("get index", getIndexResp)
			}
		} else {
			return false, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get index %s: %w", index, err))
		}
	}

	if _, found := getIndexResp.Entries[index]; found {
		return true, nil
	}
	return false, nil
}
