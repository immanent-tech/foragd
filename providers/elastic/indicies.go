// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"sync"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// CreateIndexOption is a functional option for a create index request.
type CreateIndexOption Option[*CreateIndexRequest]

func PutComponentTemplate(ctx context.Context, api *elasticsearch.TypedClient, name string, template *putcomponenttemplate.Request) error {
	_, err := api.Cluster.PutComponentTemplate(name).Request(template).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}

func PutIndexTemplate(ctx context.Context, api *elasticsearch.TypedClient, name string, template *putindextemplate.Request) error {
	_, err := api.Indices.PutIndexTemplate(name).Request(template).Do(ctx)
	if err != nil {
		return errors.Join(ErrPutILMPolicyFailed, err)
	}

	return nil
}

type CreateIndexRequest struct {
	*create.Create
	aliases map[string]types.Alias
	mu      sync.Mutex
}

// WithIndexMappings adds the given mappings to the index.
func WithIndexMappings(mappings *types.TypeMapping) CreateIndexOption {
	return func(index *CreateIndexRequest) {
		index.Mappings(mappings)
	}
}

// WithIndexSettings adds the given settings to the index.
func WithIndexSettings(settings *types.IndexSettings) CreateIndexOption {
	return func(index *CreateIndexRequest) {
		index.Settings(settings)
	}
}

// WithIndexAlias adds the given index alias to the index.
func WithIndexAlias(name string, details types.Alias) CreateIndexOption {
	return func(index *CreateIndexRequest) {
		index.mu.Lock()
		defer index.mu.Unlock()
		index.aliases[name] = details
	}
}

// NewIndexRequest creates a new index request for the given index name and options.
func NewIndexRequest(api *elasticsearch.TypedClient, name string, options ...CreateIndexOption) *create.Create {
	req := &CreateIndexRequest{
		aliases: make(map[string]types.Alias),
		Create:  api.Indices.Create(name),
	}

	for _, option := range options {
		option(req)
	}

	if len(req.aliases) > 0 {
		req.Aliases(req.aliases)
	}

	return req.Create
}
