// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
)

var (
	ErrMissingQueryParams = errors.New("missing query parameters")
	ErrInvalidQueryParams = errors.New("invalid query parameters")
	ErrRenderTemplateFail = errors.New("could not render template")
)

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Log.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
	ctx := logging.ToContext(req.Context(), logger)

	switch provider {
	case "auth0":
		auth0.LoginHandler(res, req.WithContext(ctx), s.API.auth)
	default:
		s.Log.Warn("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string, params LoginCallbackParams) {
	logger := s.Log.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
	ctx := logging.ToContext(req.Context(), logger)

	if params.Code == "" {
		logger.Error("Invalid code.")
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	if params.State == "" {
		logger.Error("Invalid state.")
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	switch provider {
	case "auth0":
		auth0.CallbackHandler(res, req.WithContext(ctx), s.API.auth, params.Code, params.State)
	default:
		logger.Warn("No provider to satisfy callback.")
		http.NotFound(res, req)
	}
	// Redirect to logged in page.
	req.Header.Add("Content-Type", "")
	http.Redirect(res, req.WithContext(ctx), "/home/show/feeds", http.StatusTemporaryRedirect)
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
