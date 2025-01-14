// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

const (
	homePage = "/home/show/feeds"
)

var (
	ErrMissingQueryParams = errors.New("missing query parameters")
	ErrInvalidQueryParams = errors.New("invalid query parameters")
	ErrRenderTemplateFail = errors.New("could not render template")
)

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

// GetLogin handles login for provider.
// (GET /login/{provider}).
func (s Server) GetLogin(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
	ctx := logging.ToContext(req.Context(), logger)

	switch provider {
	case "auth0":
		auth0.LoginHandler(res, req.WithContext(ctx), s.API.auth)
	default:
		s.Logger.Warn("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// GetLoginCallback handles callback from provider.
// (GET /login/{provider}/callback).
func (s Server) GetLoginCallback(res http.ResponseWriter, req *http.Request, provider string, params GetLoginCallbackParams) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))
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
	http.Redirect(res, req.WithContext(ctx), homePage, http.StatusTemporaryRedirect)
}

// GetLogout handles logging user out from specified provider.
// (GET /logout/{provider}).
func (s Server) GetLogout(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))

	switch provider {
	case "auth0":
		auth0.LogoutHandler(res, req)
	default:
		logger.Error("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}

// GetIndex serves the front page.
// GET(/).
func (s Server) GetIndex(res http.ResponseWriter, req *http.Request) {
	indexPage := layouts.Page("Go Feed Me",
		layouts.WithPageDescription("Welcome to Go Feed Me."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.IndexLayout()))

	// Render index page template.
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, indexPage); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("IndexViewHandler: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
