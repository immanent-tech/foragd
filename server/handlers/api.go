// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"github.com/immanent-tech/go-feed-me/providers/auth0"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/server/auth"
)

// API contains the various API backends used by handlers.
type API struct {
	User    *auth0.UserAPI
	Elastic *elastic.API
	Auth    *auth.Authenticator
}

// DataAPI returns the backend API for manipulating data.
func (a *API) DataAPI() *elastic.API {
	return a.Elastic
}

// AuthAPI returns the backend API for performing authorisation actions.
func (a *API) AuthAPI() *auth.Authenticator {
	return a.Auth
}

// UserAPI returns the backend API for managing user accounts.
func (a *API) UserAPI() *auth0.UserAPI {
	return a.User
}
