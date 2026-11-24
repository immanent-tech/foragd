/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type Doc[T ~string] interface {
	GetID() T
}

// NewRequestHeaders creates a [http.Header] object with headers that associate the OpenSearch request with any HTTP
// request that triggered it.
func NewRequestHeaders(ctx context.Context) http.Header {
	const ReqIDHeader = "X-Opaque-Id"
	headers := make(http.Header)
	headers.Set(ReqIDHeader, middleware.GetReqID(ctx))
	return headers
}

// ExtractSource extracts the `_source` field from the raw response.
func ExtractSource[T any](doc json.RawMessage) (T, error) {
	var source T
	if err := json.Unmarshal(doc, &source); err != nil {
		return source, fmt.Errorf("%w: %w", ErrExtractSource, err)
	}
	return source, nil
}
