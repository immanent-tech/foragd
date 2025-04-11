// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
)

var (
	ErrMissingQueryParams = errors.New("missing query parameters")
	ErrInvalidQueryParams = errors.New("invalid query parameters")
	ErrRenderTemplateFail = errors.New("could not render template")
)

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	switch provider {
	case "auth0":
		auth0.LoginHandler(res, req, s.API.auth)
	default:
		s.Log.Warn("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string, params LoginCallbackParams) {
	if params.Code == "" {
		slogctx.FromCtx(req.Context()).Error("Invalid Code")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.State == "" {
		slogctx.FromCtx(req.Context()).Error("Invalid State")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch provider {
	case "auth0":
		auth0.CallbackHandler(res, req, s.API.auth, params.Code, params.State)
	default:
		slogctx.FromCtx(req.Context()).Error("No Provider")
		http.NotFound(res, req)
	}
	// Redirect to logged in page.
	req.Header.Add("Content-Type", "")
	http.Redirect(res, req, "/home/feeds", http.StatusTemporaryRedirect)
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
