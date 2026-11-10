// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package opensearch

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v5"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/models"
)

var (
	// ErrExtractSource is returned when an error occurred extracting the source of a document from the response hits.
	ErrExtractSource = errors.New("could not extract document _source")
)

type Inspectable interface {
	Inspect() opensearchapi.Inspect
}

func GenerateAPIError(msg string, resp Inspectable) *models.APIError {
	err := opensearch.ParseError(resp.Inspect().Response)
	return models.NewAPIError(resp.Inspect().Response.StatusCode, fmt.Errorf("%s: %w", msg, err))
}

func HandleError(msg string, resp Inspectable, err error) *models.APIError {
	switch {
	case resp.Inspect().Response != nil:
		return GenerateAPIError(msg, resp)
	default:
		return models.NewAPIError(http.StatusInternalServerError, err)
	}
}
