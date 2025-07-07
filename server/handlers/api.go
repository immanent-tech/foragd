// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"

	"github.com/joshuar/go-feed-me/providers/auth0"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/auth"
	"github.com/joshuar/go-feed-me/server/session"
)

type API struct {
	user    *auth0.UserAPI
	elastic *elastic.API
	auth    *auth.Authenticator
}

func SetupAPI(ctx context.Context) (*API, error) {
	// Load the auth0UserAPI backend.
	auth0UserAPI, err := auth0.NewUserAPI(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to set up auth0 api: %w", err)
	}
	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to set up elastic api: %w", err)
	}
	// Set up the session manager.
	if err := session.NewSessionManager(ctx, elasticAPI, auth.SessionName); err != nil {
		return nil, fmt.Errorf("unable to set up session api: %w", err)
	}
	// Set up authentication manager.
	authAPI, err := auth.NewAuthenticator(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to set up authentication api: %w", err)
	}
	return &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		auth:    authAPI,
	}, nil
}

func (a *API) DataAPI() *elastic.API {
	return a.elastic
}

func (a *API) AuthAPI() *auth.Authenticator {
	return a.auth
}
