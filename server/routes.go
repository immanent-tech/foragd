// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"net/http"
	"slices"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

var ErrInvalidParam = errors.New("invalid parameter")

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	layout := &layouts.IndexLayout{}
	handlers.PartialRender(layout.FullRender()).ServeHTTP(res, req)
}

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger,
	).Then(handlers.PerformAuth(s.AuthAPI()))
	chain.ServeHTTP(res, req)
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger,
	).Then(handlers.AuthCallback(s.AuthAPI(), s.SessionAPI()))
	chain.ServeHTTP(res, req)
}

// GetSettings handles opening the settings modal.
func (s Server) GetSettings(res http.ResponseWriter, req *http.Request) {
	var handler http.Handler
	header := partials.Header(
		partials.DefaultHeaderStart(),
		partials.DefaultHeaderCenter(),
		partials.DefaultHeaderEnd(),
	)

	switch htmx.IsHTMX(req) {
	case true:
		handler = handlers.BaseChain.Then(
			handlers.PartialRender(
				settings.SettingsContent(),
				header,
				partials.UpdateBacklink(),
				settings.ResetFooter(),
			),
		)
	case false:
		handler = handlers.BaseChain.Then(
			handlers.FullRender("Settings",
				templates.WithBody(
					templates.NewBody(settings.SettingsContent(),
						templates.WithBodyHeader(header),
						templates.WithBodyFooter(settings.ResetFooter()),
					),
				),
			),
		)
	}

	handler.ServeHTTP(res, req)
}

func (s Server) GetTheme(res http.ResponseWriter, req *http.Request) {
	handler := handlers.BaseChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme, ok := s.SessionAPI().Get(req.Context(), models.ThemeSessionKey).(string)
		if !ok {
			slogctx.FromCtx(req.Context()).Debug("No theme in session. Using a default.")
			theme = "light"
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(theme))
	})
	handler.ServeHTTP(res, req)
}

func (s Server) SetTheme(res http.ResponseWriter, req *http.Request) {
	theme := req.FormValue("theme")
	handler := handlers.BaseChain.Append(
		handlers.SaveTheme(s.SessionAPI(), theme),
		// handlers.UpdateTheme(s.SessionAPI()),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		resp := handlers.HTMXResponseFromCtx(req.Context())
		resp.Write(res)
		res.WriteHeader(http.StatusOK)
		res.Write(nil)
	})
	handler.ServeHTTP(res, req)
}

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}

// Home handles display of the home page.
func (s Server) Home(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SaveState(s.SessionAPI()),
	).Then(handlers.DisplayHome(s.DataAPI(), s.SessionAPI()))
	chain.ServeHTTP(res, req)
}

// ShowCollection handles displaying a collection of objects, with optional filtering.
func (s Server) ShowCollection(res http.ResponseWriter, req *http.Request, collection Collection, params ShowCollectionParams) {
	// Extract any pagination value.
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	// Retrieve filters.
	filters, err := models.NewFiltersFromParams(params)
	if err != nil {
		handlers.ResponseError(res, req, &models.Response{
			StatusCode:    http.StatusInternalServerError,
			InternalError: err,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "An internal server error occurred.",
			},
		})
		return
	}
	ctx := models.FiltersToCtx(req.Context(), *filters)

	// Generate appropriate display function and view based on collection parameter.
	var (
		displayFunc http.Handler
	)
	switch collection {
	case models.CollectionSubscriptions:
		if params.Subscriptions != nil {
			displayFunc = handlers.DisplaySubscriptions(s.DataAPI(), s.SessionAPI(), pagination, *params.Subscriptions...)
		} else {
			displayFunc = handlers.DisplaySubscriptions(s.DataAPI(), s.SessionAPI(), pagination)
		}
	case models.CollectionItems:
		if params.Articles != nil {
			displayFunc = handlers.DisplayArticles(s.DataAPI(), s.SessionAPI(), pagination, *params.Subscriptions...)
		} else {
			displayFunc = handlers.DisplayArticles(s.DataAPI(), s.SessionAPI(), pagination)
		}
	default:
		handlers.ResponseError(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Collection is unknown.",
			},
		})
		return
	}
	// Generate handler chain.
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		handlers.SaveState(s.SessionAPI()),
		handlers.GenerateBacklink(s.SessionAPI()),
	).Then(displayFunc)
	// Run chain.
	chain.ServeHTTP(res, req.WithContext(ctx))
}

// ActionCollection handles performing an action on a collection of objects.
func (s Server) ActionCollection(res http.ResponseWriter, req *http.Request, collection Collection, action Action, params ActionCollectionParams) {
	var actionFunc http.Handler

	switch {
	case collection == models.CollectionSubscriptions && slices.Contains([]Action{models.ActionRead, models.ActionUnread}, action):
		// Mark feeds read/unread.
		actionFunc = handlers.MarkFeeds(s.DataAPI(), models.Mark(action), *params.Subscriptions...)
	case collection == models.CollectionItems && slices.Contains([]Action{models.ActionRead, models.ActionUnread}, action):
		// Mark items read/unread.
		actionFunc = handlers.MarkItems(s.DataAPI(), models.Mark(action), *params.Articles...)
	default:
		// Unsupported action for a collection.
		res.WriteHeader(http.StatusNotImplemented)
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(s.SessionAPI(), params.Redirect),
	).Then(actionFunc)
	chain.ServeHTTP(res, req)
}

// ActionItem handles performing an action on an item.
func (s Server) ActionArticle(res http.ResponseWriter, req *http.Request, action Action, item ItemID, params ActionArticleParams) {
	var actionFunc http.Handler

	switch action {
	case models.ActionRead, models.ActionUnread:
		actionFunc = handlers.MarkItems(s.DataAPI(), models.Mark(action), item)
	default:
		// Unimplemented action for an item.
		res.WriteHeader(http.StatusNotImplemented)
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(s.SessionAPI(), params.Redirect),
	).Then(actionFunc)
	chain.ServeHTTP(res, req)
}

// ShowItem handles showing an item.
func (s Server) ShowArticle(res http.ResponseWriter, req *http.Request, item ItemID) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SaveState(s.SessionAPI()),
		handlers.GenerateBacklink(s.SessionAPI()),
	).Then(handlers.DisplayArticle(s.DataAPI(), s.SessionAPI(), item))
	chain.ServeHTTP(res, req)
}

// ActionSubscription performs an action on a subscription.
func (s Server) ActionSubscription(res http.ResponseWriter, req *http.Request, action Action, sub SubscriptionID, params ActionSubscriptionParams) {
	var actionFunc http.Handler
	switch action {
	case models.ActionRead, models.ActionUnread:
		actionFunc = handlers.MarkSubscriptions(s.DataAPI(), models.Mark(action), sub)
	default:
		res.WriteHeader(http.StatusNotImplemented)
		return
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(s.SessionAPI(), params.Redirect),
	).Then(actionFunc)
	chain.ServeHTTP(res, req)
}

// ShowSubscription handles showing items for a feed.
func (s Server) ShowSubscription(res http.ResponseWriter, req *http.Request, sub SubscriptionID, params ShowSubscriptionParams) {
	// Extract any pagination value.
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	// Retrieve filters.
	filters, err := models.NewFiltersFromParams(params)
	if err != nil {
		handlers.ResponseError(res, req, &models.Response{
			StatusCode:    http.StatusInternalServerError,
			InternalError: err,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "An internal server error occurred.",
			},
		})
		return
	}
	ctx := models.FiltersToCtx(req.Context(), *filters)
	// Generate handler chain.
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		handlers.SaveState(s.SessionAPI()),
		handlers.GenerateBacklink(s.SessionAPI()),
	).Then(handlers.DisplayArticles(s.DataAPI(), s.SessionAPI(), pagination, sub))
	// Run chain.
	chain.ServeHTTP(res, req.WithContext(ctx))
}
