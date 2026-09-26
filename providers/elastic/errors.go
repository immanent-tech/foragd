// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/models"
)

var ErrNotFound = errors.New("not found")

func getStatusCode(err error) int {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		return esErr.Status
	}
	return http.StatusInternalServerError
}

func AsAPIError(err error) *models.APIError {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		var errStr strings.Builder
		errStr.WriteString(esErr.ErrorCause.Type)
		if esErr.ErrorCause.Reason != nil {
			errStr.WriteString(": ")
			errStr.WriteString(*esErr.ErrorCause.Reason)
		}
		return models.NewAPIError(esErr.Status, errors.New(errStr.String()))
	}
	return models.NewAPIError(http.StatusInternalServerError, err)
}
