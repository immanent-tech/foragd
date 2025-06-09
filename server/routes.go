// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/validation"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

var ErrInvalidParam = errors.New("invalid parameter")

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	alice.New(
		handlers.RouteLogger,
	).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		indexLayout := &layouts.IndexLayout{}
		page := templates.NewPage("Go Feed Me",
			indexLayout.FullRender(),
		).Show()
		if err := page.Render(req.Context(), res); err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
			http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
		}
	}).ServeHTTP(res, req)
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
		handlers.GenerateSettings,
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
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
		handlers.SavePageState,
		handlers.GenerateHomeContent(s.DataAPI()),
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
}

// ShowCollection handles displaying a collection of objects, with optional filtering.
func (s Server) ShowCollection(res http.ResponseWriter, req *http.Request, collection Collection, params ShowCollectionParams) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
	)

	var subIDs []models.SubscriptionID
	if params.Subscriptions != nil {
		subIDs = append(subIDs, *params.Subscriptions...)
	}
	switch collection {
	case models.CollectionSubscriptions:
		chain = chain.Append(handlers.GenerateSubscriptionCollection(s.DataAPI(), "", subIDs...))
	case models.CollectionArticles:
		chain = chain.Append(handlers.GenerateArticleCollection(s.DataAPI(), "", subIDs...))
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

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
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

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
	)

	switch collection {
	case models.CollectionSubscriptions:
		chain = chain.Append(handlers.PaginateSubscriptionCollection(s.DataAPI(), pagination))
	case models.CollectionArticles:
		chain = chain.Append(handlers.PaginateArticleCollection(s.DataAPI(), pagination))
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

	chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

// ActionCollection handles performing an action on a collection of objects.
func (s Server) ActionCollection(res http.ResponseWriter, req *http.Request, collection Collection, action Action, params ActionCollectionParams) {
	var actionFunc func(next http.Handler) http.Handler

	switch {
	case collection == models.CollectionSubscriptions && slices.Contains([]Action{models.ActionRead, models.ActionUnread}, action):
		// Mark feeds read/unread.
		actionFunc = handlers.MarkSubscriptions(s.DataAPI(), models.Mark(action), *params.Subscriptions...)
	case collection == models.CollectionItems && slices.Contains([]Action{models.ActionRead, models.ActionUnread}, action):
		// Mark items read/unread.
		actionFunc = handlers.MarkArticles(s.DataAPI(), models.Mark(action), *params.Articles...)
	default:
		// Unsupported action for a collection.
		res.WriteHeader(http.StatusNotImplemented)
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.Redirect),
		actionFunc,
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// ActionArticle handles performing an action on an article.
func (s Server) ActionArticle(res http.ResponseWriter, req *http.Request, action Action, item ItemID, params ActionArticleParams) {
	var actionFunc func(next http.Handler) http.Handler

	switch action {
	case models.ActionRead, models.ActionUnread:
		actionFunc = handlers.MarkArticles(s.DataAPI(), models.Mark(action), item)
	default:
		// Unimplemented action for an item.
		res.WriteHeader(http.StatusNotImplemented)
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.Redirect),
		actionFunc,
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// ShowArticle handles showing an article's content.
func (s Server) ShowArticle(res http.ResponseWriter, req *http.Request, itemID ItemID) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.GenerateArticle(s.DataAPI(), itemID),
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
}

// ShowSubscription handles showing items for a feed.
func (s Server) ShowSubscription(res http.ResponseWriter, req *http.Request, sub SubscriptionID, params ShowSubscriptionParams) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
		handlers.GenerateArticleCollection(s.DataAPI(), "", sub),
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
}

// PaginateSubscription handles paginating through the articles in a subscription.
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
	alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
		handlers.PaginateArticleCollection(s.DataAPI(), pagination, sub),
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

// ActionSubscription performs an action on a subscription.
func (s Server) ActionSubscription(res http.ResponseWriter, req *http.Request, action Action, sub SubscriptionID, params ActionSubscriptionParams) {
	var actionFunc func(next http.Handler) http.Handler
	switch action {
	case models.ActionRead, models.ActionUnread:
		actionFunc = handlers.MarkSubscriptions(s.DataAPI(), models.Mark(action), sub)
	default:
		res.WriteHeader(http.StatusNotImplemented)
		return
	}
	chain := alice.New(
		handlers.RouteLogger,
		actionFunc,
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// EditSubscription handles a request to edit a user's subscription.
func (s Server) EditSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID) {
	// Edit requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.EditSubscription(s.DataAPI(), subscriptionID),
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// SaveSubscription handles saving any edits to a user's subscription.
func (s Server) SaveSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID) {
	subscriptionEdits, valid, err := forms.DecodeForm[*models.SubscriptionCustomisation](req)
	if err != nil || !valid {
		handlers.ProcessResponse(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "There was a problem saving the subscription edits.",
			},
			InternalError: err,
		})
		return
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SaveSubscription(s.DataAPI(), subscriptionID, subscriptionEdits),
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// RemoveSubscription handles unsubscribing from a feed.
func (s Server) RemoveSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID, params RemoveSubscriptionParams) {
	// Remove requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.RemoveSubscription(s.DataAPI(), subscriptionID, params.Confirmation),
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

// NewSubscription handles presenting the user with a form to enter details about a new subscription.
func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	// Add requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.NewSubscription,
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

// AddSubscription handles an add subscription request.
func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	// Add requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden("Request is not allowed.", nil))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ParseNewSubscriptionRequest,
		handlers.ProcessSubscriptionRequests(s.DataAPI()),
		handlers.NewSubscriptionRequestResult,
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

func (s Server) StartSubscriptionImport(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.NewSubscriptionsImport,
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderPage()).ServeHTTP(res, req)
	}
}

func (f *SetSubscriptionImportMethodFormdataBody) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false, err
	}
	return true, nil
}

func (f *SetSubscriptionImportMethodFormdataBody) Sanitise() error {
	return nil
}

func (s Server) SetSubscriptionImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetSubscriptionImportMethodFormdataBody](req)
	if err != nil || !valid {
		handlers.ProcessResponse(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "There was a problem parsing the import method.",
			},
			InternalError: err,
		})
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(string(importMethod.From)),
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}

// ProcessImport performs the actions required to import requests from any source.
func (s Server) ProcessSubscriptionImport(res http.ResponseWriter, req *http.Request) {
	// Decode the import source.
	importMethod, err := forms.DecodeMultipartValue(req, "source")
	if err != nil {
		handlers.ProcessResponse(res, req, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "There was a problem parsing the import method.",
			},
			InternalError: err,
		})
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(importMethod),
		handlers.ProcessSubscriptionRequests(s.DataAPI()),
		handlers.SubscriptionsImportResults,
	).Then(handlers.RenderPartials())
	chain.ServeHTTP(res, req)
}

// GetAllSubscriptionsState handles fetching the current state of all user subscriptions.
func (s Server) GetAllSubscriptionsState(res http.ResponseWriter, req *http.Request) {
	alice.New(
		handlers.RouteLogger,
		handlers.GenerateDrawerContent(s.DataAPI()),
	).Then(handlers.RenderPartials()).ServeHTTP(res, req)
}
