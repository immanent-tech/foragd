/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
)

const (
	// indexWriteSuffix is the suffix appended to indices that are used for write (indexing) operations.
	indexWriteSuffix = "rw"
	// indexReadSuffix is the suffix appended to indices that are used for read (search, get) operations.
	indexReadSuffix = "ro"
)

// Index represents an index in OpenSearch.
type Index string

func (i Index) String() string {
	return string(i)
}

// IndexReadAlias returns the read alias for the given [Index].
func (c *Client) IndexReadAlias(name Index) string {
	return name.String() + "_" + c.environment + "_" + indexReadSuffix
}

// IndexWriteAlias returns the write alias for the given [Index].
func (c *Client) IndexWriteAlias(name Index) string {
	return name.String() + "_" + c.environment + "_" + indexWriteSuffix
}

// IndexCanonicalName returns a unique, time-stamped [Index] name that should be used for all underlying index management
// requests.
func (c *Client) IndexCanonicalName(name Index) string {
	return strings.Join(
		[]string{name.String(), c.environment, time.Now().Format("20060102150405"), "000000"},
		"-",
	)
}

// CreateIndexTemplate creates the given index template. If it already exists, it will be overridden.
func (c *Client) CreateIndexTemplate(ctx context.Context, req opensearchapi.IndicesPutIndexTemplateReq) error {
	if req.Header == nil {
		req.Header = NewRequestHeaders(ctx)
	}
	if resp, err := c.Indices.PutIndexTemplate(ctx, req); err != nil {
		return HandleError("put index template", resp, err)
	}
	return nil
}

// CreateComponentTemplate creates the given component template. If it already exists, it will be overridden.
func (c *Client) CreateComponentTemplate(ctx context.Context, req opensearchapi.ClusterPutComponentTemplateReq) error {
	if req.Header == nil {
		req.Header = NewRequestHeaders(ctx)
	}
	if resp, err := c.Cluster.PutComponentTemplate(ctx, req); err != nil {
		return HandleError("put component template", resp, err)
	}
	return nil
}

// UpdateIndexAlias will update the given index alias to point to the given index. If the alias exists, it is deleted
// and recreated. Otherwise, it is created.
func (c *Client) UpdateIndexAlias(ctx context.Context, alias, index string, asWriteIndex bool) error {
	// Get all existing aliases.
	getAliasResp, err := c.Indices.GetAlias(ctx, &opensearchapi.IndicesGetAliasReq{Header: NewRequestHeaders(ctx)})
	if err != nil {
		return HandleError("get alias", getAliasResp, err)
	}
	// If the alias is already in use, delete it.
	for aliasedIndex, aliases := range getAliasResp.Entries {
		if _, found := aliases.Aliases[alias]; found {
			deleteAliasResp, err := c.Indices.DeleteAlias(ctx, &opensearchapi.IndicesDeleteAliasReq{
				Name:    []string{alias},
				Indices: []string{aliasedIndex},
			})
			if err != nil {
				return HandleError("delete alias", deleteAliasResp, err)
			}
		}
	}

	// (Re)Create the index alias.
	createAliasResp, err := c.Indices.PutAlias(ctx, opensearchapi.IndicesPutAliasReq{
		Name: alias,
		Body: &opensearchapi.IndicesPutAliasBody{
			Index:        &index,
			IsWriteIndex: &asWriteIndex,
		},
	})
	if err != nil {
		return HandleError("create alias", createAliasResp, err)
	}

	return nil
}

// AliasExists returns a boolean indicating whether the given alias exists and points to the given index. A non-nil
// error is returned if this cannot be determined.
func (c *Client) AliasExists(ctx context.Context, alias string) (bool, error) {
	// Get the existing alias details.
	getAliasResp, err := c.Indices.GetAlias(ctx, &opensearchapi.IndicesGetAliasReq{Header: NewRequestHeaders(ctx)})
	if err != nil {
		return false, HandleError("get alias", getAliasResp, err)
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
func (c *Client) CreateIndexIfNotExists(ctx context.Context, index string) error {
	getIndexResp, err := c.Indices.Get(ctx, &opensearchapi.IndicesGetReq{
		Indices: []string{index},
		Header:  NewRequestHeaders(ctx),
	})
	if err != nil {
		if getIndexResp.Inspect().Response != nil {
			if getIndexResp.Inspect().Response.StatusCode != http.StatusNotFound {
				return GenerateAPIError("get index", getIndexResp)
			}
		} else {
			return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get index %s: %w", index, err))
		}
	}

	if _, found := getIndexResp.Entries[index]; found {
		return nil
	}

	createIndexResp, err := c.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Header: NewRequestHeaders(ctx),
		Index:  index,
	})
	if err != nil {
		return HandleError("create index", createIndexResp, err)
	}

	return nil
}
