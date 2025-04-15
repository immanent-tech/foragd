// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"net/http"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/auth"
)

var (
	ErrMissingQueryParams = errors.New("missing query parameters")
	ErrInvalidQueryParams = errors.New("invalid query parameters")
	ErrRenderTemplateFail = errors.New("could not render template")
)

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := AddRouteLogger(req.Context(), "login")
	ctx = auth.ProviderToCtx(ctx, provider)
	handlers.Login(s.AuthAPI()).ServeHTTP(res, req.WithContext(ctx))
	// switch provider {
	// case "auth0":
	// 	auth0.LoginHandler(res, req.WithContext(ctx), s.API.auth)
	// default:
	// 	slogctx.FromCtx(ctx).Warn("No provider to satisfy login.")
	// 	http.NotFound(res, req)
	// }
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string, params LoginCallbackParams) {
	ctx := AddRouteLogger(req.Context(), "login_callback")
	ctx = auth.ProviderToCtx(ctx, provider)
	handlers.AuthCallback().ServeHTTP(res, req.WithContext(ctx))

	// if params.Code == "" {
	// 	slogctx.FromCtx(ctx).Error("Invalid Code")
	// 	res.WriteHeader(http.StatusUnauthorized)
	// 	return
	// }

	// if params.State == "" {
	// 	slogctx.FromCtx(ctx).Error("Invalid State")
	// 	res.WriteHeader(http.StatusUnauthorized)
	// 	return
	// }

	// switch provider {
	// case "auth0":
	// 	auth0.CallbackHandler(res, req.WithContext(ctx), s.API.auth, params.Code, params.State)
	// default:
	// 	slogctx.FromCtx(ctx).Error("No Provider")
	// 	http.NotFound(res, req.WithContext(ctx))
	// }
	// // Redirect to logged in page.
	// req.Header.Add("Content-Type", "")
	// http.Redirect(res, req.WithContext(ctx), models.FeedsRoute, http.StatusTemporaryRedirect)
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
