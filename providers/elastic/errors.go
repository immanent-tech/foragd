// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"net/http"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

func getStatusCode(err error) int {
	if esErr, ok := errors.AsType[*types.ElasticsearchError](err); ok {
		return esErr.Status
	}
	return http.StatusInternalServerError
}
