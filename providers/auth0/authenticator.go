// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/auth0/go-auth0/v2/authentication"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/goforj/godump"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/server/session"
)

var ErrNoIDToken = errors.New("no id_token field in oauth2 token")
var ErrInvalidToken = errors.New("token is invalid")

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
}

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
			return fmt.Errorf("load config: %w", err)
		}

		provider, err := oidc.NewProvider(
			ctx,
			"https://"+cfg.Domain+"/",
		)
		if err != nil {
			return fmt.Errorf("create provider: %w", err)
		}

		conf := oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email"},
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

	returnTo, err := url.Parse("https://" + req.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to generate logout URL: %w", err)
	}

	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", cfg.ClientID)
	logoutURL.RawQuery = parameters.Encode()

	return logoutURL, nil
}

func RefreshAccessToken(res http.ResponseWriter, req *http.Request, currentToken *oauth2.Token) error {
	if err := InitAuthenticator(req.Context()); err != nil {
		return fmt.Errorf("unable to generate logout URL: %w", err)
	}

	godump.Dump(currentToken)

	// Generate API url for refreshing the token.
	refreshURL, err := url.Parse("https://" + cfg.Domain + "/oauth/token")
	if err != nil {
		return fmt.Errorf("unable to generate logout url: %w", err)
	}

	// Add parameters.
	parameters := url.Values{}
	parameters.Add("grant_type", "refresh_token")
	parameters.Add("client_id", cfg.ClientID)
	parameters.Add("client_secret", cfg.ClientSecret)
	parameters.Add("refresh_token", currentToken.RefreshToken)
	payload := strings.NewReader(parameters.Encode())
	godump.Dump(payload)

	var newToken oauth2.Token
	var errResult authentication.Error

	client := loadHTTPClient()
	if resp, err := client.R().
		SetBody(payload).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetResult(&newToken).
		SetError(&errResult).
		Post(refreshURL.String()); err != nil || resp.IsError() {
		slogctx.FromCtx(req.Context()).Error("Unable to refresh session token.",
			slog.Any("error", &errResult),
		)
		// Generate new state and save url for redirection after login.
		if state, err := GenerateRandomState(); err != nil {
			slogctx.FromCtx(req.Context()).Error("Generate new state failed.",
				slog.Any("error", err),
			)
		} else {
			session.Save(req.Context(), "state", state)
			session.Save(req.Context(), state, map[string]string{
				"redirectURL": req.URL.String(),
			})
		}
		http.Redirect(res, req, "/login", http.StatusSeeOther)
	}

	slogctx.FromCtx(req.Context()).Debug("Refreshed access token.")

	session.Save(req.Context(), "token", newToken)

	return nil

}
