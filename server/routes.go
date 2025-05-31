// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
)

var ErrInvalidParam = errors.New("invalid parameter")

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.GenerateIndex,
	).Then(handlers.RenderTemplates())
	chain.ServeHTTP(res, req)
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
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.GenerateSettings,
	).Then(handlers.RenderTemplates())
	chain.ServeHTTP(res, req)
}

// GetTheme retrieves the user's chosen theme.
func (s Server) GetTheme(res http.ResponseWriter, req *http.Request) {
	handler := handlers.BaseChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user, found := models.UserFromCtx(req.Context())
		if !found {
			handlers.ProcessResponse(res, req, models.RespInvalidUser())
			return
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(user.GetSettings().Theme))
	})
	handler.ServeHTTP(res, req)
}

// SetTheme saves the user's chosen theme.
func (s Server) SetTheme(res http.ResponseWriter, req *http.Request) {
	theme := req.FormValue("theme")
	user, found := models.UserFromCtx(req.Context())
	if !found {
		handlers.ProcessResponse(res, req, models.RespInvalidUser())
		return
	}
	settings := user.GetSettings()
	settings.Theme = theme
	resp := s.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
		"settings":   settings,
		"updated_at": time.Now().UTC(),
	})
	if resp.IsError() {
		handlers.ProcessResponse(res, req, resp)
		res.WriteHeader(http.StatusNoContent)
	} else {
		res.WriteHeader(http.StatusOK)
	}
}

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}

// Home handles display of the home page.
func (s Server) Home(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.GenerateHomeContent(s.DataAPI()),
	).Then(handlers.RenderTemplates())
	chain.ServeHTTP(res, req)
}

// ShowCollection handles displaying a collection of objects, with optional filtering.
func (s Server) ShowCollection(res http.ResponseWriter, req *http.Request, collection Collection, params ShowCollectionParams) {
	spew.Dump(chi.RouteContext(req.Context()).URLParams)
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
			handlers.GenerateSubscriptionCollection,
		).Then(handlers.RenderTemplates())
	case models.CollectionArticles:
		displayFunc = baseChain.Append(
			handlers.FetchArticles(s.DataAPI(), "", subIDs...),
			handlers.GenerateArticleCollection,
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

// UpdateCollection handles updating the display of a collection of objects after changing filters.
func (s Server) UpdateCollection(res http.ResponseWriter, req *http.Request, collection Collection, params UpdateCollectionParams) {
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
			handlers.GenerateSubscriptionCollection,
		).Then(handlers.RenderTemplates())
	case models.CollectionArticles:
		displayFunc = baseChain.Append(
			handlers.FetchArticles(s.DataAPI(), pagination),
			handlers.GenerateArticleCollection,
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
func (s Server) ShowArticle(res http.ResponseWriter, req *http.Request, itemID ItemID) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetResponseHeaders,
		handlers.SavePageState,
		handlers.GenerateArticle(s.DataAPI(), itemID),
	).Then(handlers.RenderTemplates())
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
		handlers.GenerateArticleCollection,
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
		handlers.FetchArticles(s.DataAPI(), pagination, sub),
		handlers.GenerateArticleCollection,
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
