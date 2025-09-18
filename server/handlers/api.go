// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"github.com/immanent-tech/go-feed-me/providers/elastic"
)

// API contains the various API backends used by handlers.
type API struct {
	Elastic *elastic.API
}

// DataAPI returns the backend API for manipulating data.
func (a *API) DataAPI() *elastic.API {
	return a.Elastic
}
