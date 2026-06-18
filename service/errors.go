// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/models"
)

// ElasticsearchToAPIError will extract and wrap a types.ElasticsearchError from the given error, in an APIError
// containing its pertinent information. If the given error does not contain types.ElasticsearchError, the given error
// is wrapped in a generic APIError is created.
func ElasticsearchToAPIError(err error) error {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		var str strings.Builder

		str.WriteString(*esErr.ErrorCause.Reason)
		str.WriteString(" (")
		str.WriteString(esErr.ErrorCause.Type)
		str.WriteString(")")
		if esErr.ErrorCause.RootCause != nil {
			str.WriteString(" reason: ")
			str.WriteString(*esErr.ErrorCause.CausedBy.Reason)
		}

		return &models.APIError{
			InternalError: fmt.Errorf("%s", str.String()),
			StatusCode:    esErr.Status,
		}
	}
	return &models.APIError{
		InternalError: fmt.Errorf("%w: %w", models.ErrInvalidAPIResult, err),
		StatusCode:    http.StatusInternalServerError,
	}
}
