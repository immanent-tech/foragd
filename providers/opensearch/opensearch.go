/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/immanent-tech/go-base/config"
	"github.com/opensearch-project/opensearch-go/v5"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

// NewRequestHeaders creates a [http.Header] object with headers that associate the OpenSearch request with any HTTP
// request that triggered it.
func NewRequestHeaders(ctx context.Context) http.Header {
	const ReqIDHeader = "X-Opaque-Id"
	headers := make(http.Header)
	headers.Set(ReqIDHeader, middleware.GetReqID(ctx))
	return headers
}

// Client hold the underlying OpenSearch client and provides methods for interacting with OpenSearch.
type Client struct {
	*opensearchapi.Client

	environment string
}

var GetClient = sync.OnceValues(func() (*Client, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}

	cfg, err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses:          cfg.Hosts,
			Username:           cfg.User,
			Password:           cfg.Password,
			InsecureSkipVerify: cfg.SkipTLSVerification,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return &Client{Client: client, environment: appCfg.GetAppEnvironment().String()}, nil
})
