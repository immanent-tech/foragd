// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"fmt"
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
	"github.com/joshuar/go-feed-me/web/views"
)

var ErrInvalidParam = errors.New("invalid parameter")

// SignUp handles presenting a form for the user to enter sign-up details.
func (s Server) SignUp(res http.ResponseWriter, req *http.Request) {
	alice.New(
		handlers.NewUserSignup,
	).Then(handlers.RenderPage()).ServeHTTP(res, req)
}

// ProcessSignUp handles validating and processing a user sign-up request.
func (s Server) ProcessSignUp(res http.ResponseWriter, req *http.Request) {
	resp := htmx.NewResponse()
	// Decode and validate the user sign-up request.
	userSignup, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
	if err != nil || !valid {
		slogctx.FromCtx(req.Context()).Debug("Problem decoding user signup form data.",
			slog.Bool("valid", valid),
			slog.Any("error", err),
		)
		if err := resp.RenderTempl(req.Context(), res, views.SignupForm(userSignup)); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Bad request.", slog.Any("error", err))
			http.Error(res, "user signup failed!", http.StatusInternalServerError)
		}
		return
	}
	// Process the sign-up request.
	alice.New(
		handlers.RouteLogger,
		handlers.ProcessUserSignup(s.UserAPI(), s.DataAPI(), userSignup),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

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
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
	}
}

// GetTheme retrieves the user's chosen theme.
func (s Server) GetTheme(res http.ResponseWriter, req *http.Request) {
	handler := handlers.BaseChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user, found := models.UserFromCtx(req.Context())
		if !found {
			handlers.ProcessResponse(res, req, models.RespErrUnauthorized())
			return
		}
		res.WriteHeader(http.StatusOK)
		if _, err := res.Write([]byte(user.GetSettings().Theme)); err != nil {
			handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		}
	})
	handler.ServeHTTP(res, req)
}

// SetTheme saves the user's chosen theme.
func (s Server) SetTheme(res http.ResponseWriter, req *http.Request) {
	theme := req.FormValue("theme")
	user, found := models.UserFromCtx(req.Context())
	if !found {
		handlers.ProcessResponse(res, req, models.RespErrUnauthorized())
		return
	}
	settings := user.GetSettings()
	settings.Theme = theme
	if err := s.DataAPI().UpdateUser(req.Context(), map[string]any{
		"settings":   settings,
		"updated_at": time.Now().UTC(),
	}); err != nil {
		handlers.ProcessResponse(res, req, models.NewResponse(http.StatusNoContent, fmt.Errorf("failed to update theme: %w", err)))
	}
	s.GetSettings(res, req)
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
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
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
		chain = chain.Append(handlers.GenerateSubscriptionCollection(s.DataAPI(), subIDs...))
	case models.CollectionArticles:
		chain = chain.Append(handlers.GenerateArticleCollection(s.DataAPI(), subIDs...))
	default:
		handlers.ProcessResponse(res, req, models.RespInvalidInput())
		return
	}

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
	}
}

// UpdateCollection handles updating the display of a collection of objects after changing filters.
func (s Server) UpdateCollection(res http.ResponseWriter, req *http.Request, collection Collection, params UpdateCollectionParams) {
}

// PaginateCollection handles displaying a collection of objects, with optional filtering.
func (s Server) PaginateCollection(res http.ResponseWriter, req *http.Request, collection Collection, params PaginateCollectionParams) {
	// Pagination requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
	)

	switch collection {
	case models.CollectionSubscriptions:
		chain = chain.Append(handlers.PaginateSubscriptionCollection(s.DataAPI()))
	case models.CollectionArticles:
		chain = chain.Append(handlers.PaginateArticleCollection(s.DataAPI()))
	default:
		handlers.ProcessResponse(res, req, models.RespInvalidInput())
		return
	}

	chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
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
	).Then(handlers.RenderContentPartials())
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
	).Then(handlers.RenderContentPartials())
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
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
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
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
	}
}

// PaginateSubscription handles paginating through the articles in a subscription.
func (s Server) PaginateSubscription(res http.ResponseWriter, req *http.Request, sub SubscriptionID, params PaginateSubscriptionParams) {
	// Pagination requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}
	alice.New(
		handlers.RouteLogger,
		handlers.SavePageState,
		handlers.SaveFilters(params),
		handlers.PaginateArticleCollection(s.DataAPI(), sub),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
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
	).Then(handlers.RenderContentPartials())
	chain.ServeHTTP(res, req)
}

// EditSubscription handles a request to edit a user's subscription.
func (s Server) EditSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID) {
	// Edit requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.EditSubscription(s.DataAPI(), subscriptionID),
	).Then(handlers.RenderContentPartials())
	chain.ServeHTTP(res, req)
}

// SaveSubscription handles saving any edits to a user's subscription.
func (s Server) SaveSubscription(res http.ResponseWriter, req *http.Request, _ models.SubscriptionID) {
	edits, valid, err := forms.DecodeForm[*models.SubscriptionEdit](req)
	if err != nil || !valid {
		handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		return
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SaveSubscription(s.DataAPI(), edits),
	).Then(handlers.RenderContentPartials())
	chain.ServeHTTP(res, req)
}

// RemoveSubscription handles unsubscribing from a feed.
func (s Server) RemoveSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID, params RemoveSubscriptionParams) {
	// Remove requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.RemoveSubscription(s.DataAPI(), subscriptionID, params.Confirmation),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// NewSubscription handles presenting the user with a form to enter details about a new subscription.
func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	// Add requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.NewSubscription,
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// AddSubscription handles an add subscription request.
func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	// Add requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ParseNewSubscriptionRequest,
		handlers.MatchFeedsToSubscriptionRequests(s.DataAPI()),
		handlers.AddFeedsForSubscriptionRequests(s.DataAPI()),
		handlers.AddSubscriptions(s.DataAPI()),
		handlers.NewSubscriptionRequestResult,
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// StartSubscriptionImport handles starting a subscriptions import process for the user.
func (s Server) StartSubscriptionImport(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.NewSubscriptionsImport,
	)

	switch htmx.IsHTMX(req) {
	case true:
		chain.Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
	case false:
		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderContentPage()).ServeHTTP(res, req)
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

// SetSubscriptionImportMethod handles setting the method that will be used for importing susbcriptions from the user's
// choice.
func (s Server) SetSubscriptionImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetSubscriptionImportMethodFormdataBody](req)
	if err != nil || !valid {
		handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(string(importMethod.From)),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// ProcessSubscriptionImport handles using the user's chosen import method to import their subscriptions.
func (s Server) ProcessSubscriptionImport(res http.ResponseWriter, req *http.Request) {
	// Decode the import source.
	importMethod, err := forms.DecodeMultipartValue(req, "source")
	if err != nil {
		handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(importMethod),
		handlers.MatchFeedsToSubscriptionRequests(s.DataAPI()),
		handlers.AddFeedsForSubscriptionRequests(s.DataAPI()),
		handlers.AddSubscriptions(s.DataAPI()),
		handlers.SubscriptionsImportResults,
	).Then(handlers.RenderContentPartials())
	chain.ServeHTTP(res, req)
}

// GetAllSubscriptionsState handles fetching the current state of all user subscriptions.
func (s Server) GetAllSubscriptionsState(res http.ResponseWriter, req *http.Request) {
	alice.New(
		handlers.RouteLogger,
		handlers.GenerateDrawerContent(s.DataAPI()),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// SearchSuggest handles processing and displaying a list of search suggestions.
func (s Server) SearchSuggest(res http.ResponseWriter, req *http.Request) {
	searchTerms := req.FormValue("search_terms")
	if searchTerms == "" {
		res.WriteHeader(http.StatusOK)
		return
	}
	alice.New(
		handlers.RouteLogger,
		handlers.GenerateSearchSuggestions(s.DataAPI(), searchTerms),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}

// SearchResults handles processing and displaying a page of search results.
func (s Server) SearchResults(res http.ResponseWriter, req *http.Request) {
	searchTerms := req.FormValue("search_terms")
	if searchTerms == "" {
		res.WriteHeader(http.StatusOK)
		return
	}
	alice.New(
		handlers.RouteLogger,
		handlers.GenerateSearchResults(s.DataAPI(), searchTerms),
	).Then(handlers.RenderContentPartials()).ServeHTTP(res, req)
}
