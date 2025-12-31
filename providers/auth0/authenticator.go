// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/oauth2"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/immanent-tech/foragd/config"
)

var ErrNoIDToken = errors.New("no id_token field in oauth2 token")

// Authenticator is used to authenticate our users.
type Authenticator struct {
	*oidc.Provider
	oauth2.Config
}

var AuthClient *Authenticator

// InitAuthenticator will the setup and initialisation of the Auth0 tenant. It can be called multiple times but will only
// perform initialisation once (so it can be lazily loaded by calling it before any Auth0 actions).
var InitAuthenticator = func(ctx context.Context) error {
	err := sync.OnceValue(func() error {
		err := loadConfigOnce()
		if err != nil {
			return fmt.Errorf("unable to create authenticator: %w", err)
		}

		provider, err := oidc.NewProvider(
			ctx,
			"https://"+cfg.Domain+"/",
		)
		if err != nil {
			return fmt.Errorf("unable to create authenticator: %w", err)
		}

		conf := oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}
		AuthClient = &Authenticator{
			Provider: provider,
			Config:   conf,
		}
		return nil
	})()
	if err != nil {
		return err
	}
	return nil
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, ErrNoIDToken
	}
	oidcConfig := &oidc.Config{
		ClientID: AuthClient.ClientID,
	}
	id, err := AuthClient.Verifier(oidcConfig).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("unable to verify token: %w", err)
	}
	return id, nil
}

// GenerateLogoutURL generates URL to log the user out from the auth backend.
func GenerateLogoutURL(req *http.Request) (*url.URL, error) {
	if err := InitAuthenticator(req.Context()); err != nil {
		return nil, fmt.Errorf("unable to generate logout URL: %w", err)
	}
	logoutURL, err := url.Parse("https://" + cfg.Domain + "/v2/logout")
	if err != nil {
		return nil, fmt.Errorf("unable to generate logout url: %w", err)
	}
	scheme := "http"
	if config.CurrentEnvironment == config.EnvProduction {
		scheme = "https"
	}

	returnTo, err := url.Parse(scheme + "://" + req.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to generate logout URL: %w", err)
	}

	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", cfg.ClientID)
	logoutURL.RawQuery = parameters.Encode()

	return logoutURL, nil
}
