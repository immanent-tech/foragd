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

	"github.com/joshuar/go-feed-me/components/session"
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
	).Then(handlers.AuthCallback(s.AuthAPI()))
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
		theme, ok := session.Manager.Get(req.Context(), models.ThemeSessionKey).(string)
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
		handlers.SaveTheme(theme),
		// handlers.UpdateTheme(session.Manager),
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
		handlers.SavePageState,
	).Then(handlers.DisplayHome(s.DataAPI()))
	chain.ServeHTTP(res, req)
}

// ShowCollection handles displaying a collection of objects, with optional filtering.
func (s Server) ShowCollection(res http.ResponseWriter, req *http.Request, collection Collection, params ShowCollectionParams) {
	baseChain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.SaveFilters(params),
	)

	var subIDs []models.SubscriptionID
	if params.Subscriptions != nil {
		subIDs = append(subIDs, *params.Subscriptions...)
	}

	var displayFunc http.Handler
	switch collection {
	case models.CollectionSubscriptions:
		displayFunc = baseChain.Append(
			handlers.FetchSubscriptions(s.DataAPI(), "", subIDs...),
			handlers.GenerateSubscriptionsContent,
		).Then(handlers.RenderTemplates())
	case models.CollectionArticles:
		displayFunc = baseChain.Append(
			handlers.FetchArticles(s.DataAPI(), "", subIDs...),
			handlers.GenerateArticleContent,
		).Then(handlers.RenderTemplates())
	default:
		handlers.ProcessResponse(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Collection is unknown.",
			},
		})
		return
	}

	displayFunc.ServeHTTP(res, req)
}

// PaginateCollection handles displaying a collection of objects, with optional filtering.
func (s Server) PaginateCollection(res http.ResponseWriter, req *http.Request, collection Collection, params PaginateCollectionParams) {
	// Pagination requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}
	// Extract any pagination value.
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}

	baseChain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.SaveFilters(params),
	)

	var displayFunc http.Handler
	switch collection {
	case models.CollectionSubscriptions:
		displayFunc = baseChain.Append(
			handlers.FetchSubscriptions(s.DataAPI(), pagination),
			handlers.GenerateSubscriptionsContent,
		).Then(handlers.RenderTemplates())
	case models.CollectionArticles:
		displayFunc = baseChain.Append(
			handlers.FetchArticles(s.DataAPI(), pagination),
			handlers.GenerateArticleContent,
		).Then(handlers.RenderTemplates())
	default:
		handlers.ProcessResponse(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Collection is unknown.",
			},
		})
		return
	}

	displayFunc.ServeHTTP(res, req)
}

// ActionCollection handles performing an action on a collection of objects.
func (s Server) ActionCollection(res http.ResponseWriter, req *http.Request, collection Collection, action Action, params ActionCollectionParams) {
	var actionFunc http.Handler

	switch {
	case collection == models.CollectionSubscriptions && slices.Contains([]Action{models.ActionRead, models.ActionUnread}, action):
		// Mark feeds read/unread.
		actionFunc = handlers.MarkSubscriptions(s.DataAPI(), models.Mark(action), *params.Subscriptions...)
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
		handlers.SetResponseHeaders,
		handlers.SetupRedirect(params.Redirect),
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
		handlers.SetupRedirect(params.Redirect),
	).Then(actionFunc)
	chain.ServeHTTP(res, req)
}

// ShowItem handles showing an item.
func (s Server) ShowArticle(res http.ResponseWriter, req *http.Request, item ItemID) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
	).Then(handlers.DisplayArticle(s.DataAPI(), item))
	chain.ServeHTTP(res, req)
}

// ShowSubscription handles showing items for a feed.
func (s Server) ShowSubscription(res http.ResponseWriter, req *http.Request, sub SubscriptionID, params ShowSubscriptionParams) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.SaveFilters(params),
		handlers.FetchArticles(s.DataAPI(), "", sub),
		handlers.GenerateArticleContent,
	).Then(handlers.RenderTemplates())
	// Run chain.
	chain.ServeHTTP(res, req)
}

func (s Server) PaginateSubscription(res http.ResponseWriter, req *http.Request, sub SubscriptionID, params PaginateSubscriptionParams) {
	// Pagination requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}
	// Extract any pagination value.
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.SaveFilters(params),
		handlers.FetchArticles(s.DataAPI(), pagination),
		handlers.GenerateArticleContent,
	).Then(handlers.RenderTemplates())

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
	).Then(actionFunc)
	chain.ServeHTTP(res, req)
}
