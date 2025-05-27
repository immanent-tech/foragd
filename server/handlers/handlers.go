// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/templates"
)

var (
	// ErrInvalidUser indicates the user data is invalid. This might be the case if the retrieved session is corrupted.
	ErrInvalidUser = errors.New("user data is invalid")
	// ErrMissingRequestData indicates data was expected in the request (usually in the context) but was not found.
	ErrMissingRequestData = errors.New("request data is missing")
)

var BaseChain = alice.New(
	RouteLogger,
)

// Keys for objects stored within the context and passed between handlers.
const (
	subscriptionRequestsCtxKey contextKey = "subscriptionRequests"
	subscriptionsCtxKey        contextKey = "subscriptions"
	feedsCtxKey                contextKey = "feeds"
	htmxRespCtxKey             contextKey = "htmxResp"
)

// Keys for objects stored within the session.
const (
	feedFiltersSessionKey = "feed_filters"
	itemFiltersSessionKey = "item_filters"
	LastViewedSessionKey  = "last_viewed"
)

type contextKey string

// DataAPI represents the API surface for interacting with the database/datastore backend.
type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	// Subscription methods:
	GetSubscription(ctx context.Context, subscriptionID models.SubscriptionID) (*models.Subscription, error)
	GetSubscriptions(ctx context.Context, filters models.Filters, pagination models.Pagination) (models.Subscriptions, models.Pagination, error)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) error
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) error
	// Feeds methods:
	// GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context, filters models.Filters, pagination models.Pagination) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, mark models.Mark, itemIDs ...models.ItemID) error
	GetTopItemCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, error)
}

// AuthAPI represents the API surface for interacting with the auth backend.
type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) models.UserID
}

type HXLocation struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// HTMXResponseToCtx adds the given htmx.Response object to the context.
func HTMXResponseToCtx(ctx context.Context, resp htmx.Response) context.Context {
	return context.WithValue(ctx, htmxRespCtxKey, resp)
}

// HTMXResponseFromCtx fetches the a htmx.Response object from the context. If no object exists, a new htmx.Response is
// returned.
func HTMXResponseFromCtx(ctx context.Context) htmx.Response {
	resp, found := ctx.Value(htmxRespCtxKey).(htmx.Response)
	if !found {
		slogctx.FromCtx(ctx).Warn("No existing htmx response object, creating new one.")
		return htmx.NewResponse()
	}
	return resp
}

// FullRender renders a full page with the given title and options.
func FullRender(title string, pageOptions ...templates.PageOption) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		slogctx.FromCtx(req.Context()).Debug("Performing full page render.")
		page := templates.NewPage(title, pageOptions...)
		if err := page.Template().Render(req.Context(), res); err != nil {
			InternalServerError(res, req, err)
		}
	})
}

// PartialRender will return a handler that will render the given templates via a htmx response.
func PartialRender(templates ...templ.Component) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			slogctx.FromCtx(req.Context()).Debug("Performing partial renders.")
			if !htmx.IsHTMX(req) {
				slogctx.FromCtx(req.Context()).Warn("Partial render for non-HTMX request.")
			}
			resp := HTMXResponseFromCtx(req.Context())
			for template := range slices.Values(templates) {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
				}
			}
		})
}

// SaveState saves the current page state in the session.
func SaveState(api models.SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Save page state.
			state := models.PageState{Path: req.URL.Path, Params: req.URL.Query()}
			models.SavePageStateInSession(req.Context(), api, state)
			slogctx.FromCtx(req.Context()).Debug("Saved page state.")
			// Pass control to next handler.
			next.ServeHTTP(res, req)
		})
	}
}

// GenerateView creates the view state for this page.
func GenerateView(params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			// Retrieve filters.
			filters, err := models.NewFiltersFromParams(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			ctx = models.FiltersToCtx(ctx, *filters)
			view := models.PageState{Path: req.URL.Path, Params: req.URL.Query()}
			ctx = models.ViewToCtx(req.Context(), view)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateBacklink creates the appropriate backlink for the current page view.
func GenerateBacklink(api models.SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var backlink models.PageState
			route := chi.RouteContext(req.Context()).RoutePattern()
			switch {
			case strings.HasPrefix(route, "/home/feeds"):
				backlink = models.PageState{Path: "/home"}
			case strings.HasPrefix(route, "/feed"):
				backlink = models.RestorePageStateFromSession(req.Context(), api, "/home/feeds")
			case strings.HasPrefix(route, "/item"):
				backlink = models.RestorePageStateFromSession(req.Context(), api, "/feed")
			}
			// Save backlink into context.
			ctx := models.BacklinkToCtx(req.Context(), backlink)
			slogctx.FromCtx(ctx).Debug("Generated backlink.", slog.String("backlink", backlink.String()))
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(api models.SessionAPI, path *string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if htmx.IsHTMX(req) && path != nil {
				slogctx.FromCtx(req.Context()).Debug("Setting-up client-side redirect.",
					slog.String("path", string(*path)),
				)
				view := models.RestorePageStateFromSession(req.Context(), api, *path)
				HxLocationData := HXLocation{Path: view.String(), Target: templates.ContentID.Target()}
				data, err := json.Marshal(HxLocationData)
				if err != nil {
					InternalServerError(res, req, err)
				}
				// Set-up client-side redirect to view.
				res.Header().Add(htmx.HeaderLocation, string(data))
			}
			next.ServeHTTP(res, req)
		})
	}
}

// SaveTheme handles saving the theme in the session.
func SaveTheme(api models.SessionAPI, theme string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			api.Put(req.Context(), models.ThemeSessionKey, theme)
			next.ServeHTTP(res, req)
		})
	}
}

// UpdateTheme handles firing an event trigger as part of the response to update the page theme.
func UpdateTheme(api models.SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			theme, ok := api.Get(req.Context(), models.ThemeSessionKey).(string)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("Could not retrieve theme from session.")
				next.ServeHTTP(res, req)
			}
			slogctx.FromCtx(req.Context()).Debug("Setting theme.",
				slog.String("theme", theme),
			)
			resp := HTMXResponseFromCtx(req.Context())
			resp = resp.AddTrigger(htmx.TriggerDetail("setTheme", theme))
			ctx := HTMXResponseToCtx(req.Context(), resp)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
