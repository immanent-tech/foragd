// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
)

// InternalPaginationCount defines the number of docs to retrieve in a pagination request.
const InternalPaginationCount = 1000

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*typedapi.API
	logger *slog.Logger
}

// Log can be used to write log messages decorated by the API.
func (a *API) Log() *slog.Logger {
	return a.logger
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *typedapi.API {
	return a.API
}
