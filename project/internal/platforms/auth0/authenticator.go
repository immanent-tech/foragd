// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/coreos/go-oidc"
	"github.com/knadh/koanf/v2"
	"golang.org/x/oauth2"
)

var ErrNoIDToken = errors.New("no id_token field in oauth2 token")

// Authenticator is used to authenticate our users.
type Authenticator struct {
	*oidc.Provider
	oauth2.Config
	settings settings
}

// NewAuthenticator instantiates the *Authenticator.
func NewAuthenticator(ctx context.Context, config *koanf.Koanf) (*Authenticator, error) {
	settings := getSettings(config)

	provider, err := oidc.NewProvider(
		ctx,
		"https://"+settings.Domain+"/",
	)
	if err != nil {
		return nil, fmt.Errorf("could not create new authentication provider: %w", err)
	}

	conf := oauth2.Config{
		ClientID:     settings.ClientID,
		ClientSecret: settings.ClientSecret,
		RedirectURL:  "http://localhost:" + strconv.Itoa(settings.ServerPort) + "/login/auth0/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
	}

	return &Authenticator{
		Provider: provider,
		Config:   conf,
		settings: settings,
	}, nil
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, ErrNoIDToken
	}

	oidcConfig := &oidc.Config{
		ClientID: a.ClientID,
	}

	validToken, err := a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return validToken, nil
}

func (a *Authenticator) LogoutURL(req *http.Request) (*url.URL, error) {
	logoutURL, err := url.Parse("https://" + a.settings.Domain + "/v2/logout")
	if err != nil {
		return nil, fmt.Errorf("could not determine logout URL: %w", err)
	}

	scheme := "http"
	// if req.TLS != nil {
	// 	scheme = "https"
	// }

	returnTo, err := url.Parse(scheme + "://" + req.Host)
	if err != nil {
		return nil, fmt.Errorf("invalid return host: %w", err)
	}

	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", a.settings.ClientID)
	logoutURL.RawQuery = parameters.Encode()

	return logoutURL, nil
}
