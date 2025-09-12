// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"fmt"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/go-feed-me/providers/elastic/results"
)

// SearchResponse represents a response from the `_search` API endpoint.
type SearchResponse struct {
	*search.Response
}

func ExtractDocs[T any](resp *SearchResponse) ([]T, []types.FieldValue, error) {
	docs, newSearchAfter, err := results.ExtractSourceFromHits[T](resp.Hits.Hits)
	if err != nil {
		return docs, newSearchAfter, fmt.Errorf("unable to extract docs: %w", err)
	}
	return docs, newSearchAfter, nil
}
