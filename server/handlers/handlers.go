// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/session"
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
	articlesCtxKey             contextKey = "articles"
	paginationCtxKey           contextKey = "pagination"
	templatesCtxKey            contextKey = "templates"
	feedsCtxKey                contextKey = "feeds"
	htmxRespCtxKey             contextKey = "htmxResp"
)

// Keys for objects stored within the session.
const (
	subscriptionsPageState = "subscriptions_state"
	articlesPageState      = "articles_state"
)

type contextKey string

// DataAPI represents the API surface for interacting with the database/datastore backend.
type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	// Subscription methods:
	GetSubscription(ctx context.Context, subscriptionID models.SubscriptionID) (*models.Subscription, *models.Response)
	GetSubscriptionsByID(ctx context.Context, filters models.Filters, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Subscriptions, models.Pagination, *models.Response)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) *models.Response
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) *models.Response
	EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) *models.Response
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) *models.Response
	// Feeds methods:
	// GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetArticle(ctx context.Context, itemID models.ItemID) (*models.Article, bool, *models.Response)
	GetArticlesBySubscription(ctx context.Context, filters models.Filters, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Articles, models.Pagination, *models.Response)
	MarkItems(ctx context.Context, mark models.Mark, itemIDs ...models.ItemID) *models.Response
	GetTopItemCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, *models.Response)
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

func init() {
	gob.Register(models.PageState{})
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
			ProcessResponse(res, req, &models.Response{
				StatusCode:    http.StatusInternalServerError,
				InternalError: err,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Could not render content.",
				},
			})
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

// RenderTemplates will render any templates found in the context. If none are found, it returns a 204 response.
func RenderTemplates() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			templates, ok := req.Context().Value(templatesCtxKey).([]templ.Component)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("No templates found in context.")
				res.WriteHeader(http.StatusNoContent)
				return
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
func SaveFilters(params any, collection models.Collection) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve filters.
			var (
				filters *models.Filters
				err     error
			)
			filters, err = models.NewFiltersFromParams(params)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to extract filters from params.",
					slog.Any("error", err),
				)
				filters = models.NewFilters()
			}
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SaveState saves the current page state in the session.
func SetResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Vary", "HX-Request")
		next.ServeHTTP(res, req)
	})
}

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(path *string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if htmx.IsHTMX(req) && path != nil {
				slogctx.FromCtx(req.Context()).Debug("Setting-up client-side redirect.",
					slog.String("path", *path),
				)
				view := RestorePageState(req.Context(), *path)
				// view := models.GetPreviousViewedPage(req.Context(), api)
				HxLocationData := HXLocation{Path: view.String(), Target: templates.ContentID.Target()}
				data, err := json.Marshal(HxLocationData)
				if err != nil {
					ProcessResponse(res, req, &models.Response{
						StatusCode:    http.StatusInternalServerError,
						InternalError: err,
						UserMessage: &models.UserMessage{
							Status:  models.UserMessageStatusError,
							Summary: "Redirection failed.",
						},
					})
				}
				// Set-up client-side redirect to view.
				res.Header().Add(htmx.HeaderLocation, string(data))
			}
			next.ServeHTTP(res, req)
		})
	}
}

// SaveTheme handles saving the theme in the session.
func SaveTheme(theme string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			session.Manager.Put(req.Context(), models.ThemeSessionKey, theme)
			next.ServeHTTP(res, req)
		})
	}
}

// UpdateTheme handles firing an event trigger as part of the response to update the page theme.
func UpdateTheme() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			theme, ok := session.Manager.Get(req.Context(), models.ThemeSessionKey).(string)
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

// SavePageState saves the current page state in the session.
func SavePageState(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Save page state.
		state := models.PageState{Path: req.URL.Path, Params: req.URL.Query()}
		ctx := models.PageStateToCtx(req.Context(), state)
		// Store page states for some paths into session for history restoration.
		if req.Method == http.MethodGet {
			switch req.URL.Path {
			case "/home/subscriptions":
				session.Manager.Put(req.Context(), subscriptionsPageState, state)
			case "/home/articles":
				session.Manager.Put(req.Context(), articlesPageState, state)
			}
		}
		slogctx.FromCtx(ctx).Debug("Saved page state.")
		// Pass control to next handler.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func RestorePageState(ctx context.Context, path string) models.PageState {
	switch path {
	case "/home/subscriptions":
		if state, ok := session.Manager.Get(ctx, subscriptionsPageState).(models.PageState); ok {
			return state
		}
		return models.PageState{Path: "/home/subscriptions", Params: models.NewFilters().ToQueryParams()}
	case "/home/articles":
		if state, ok := session.Manager.Get(ctx, articlesPageState).(models.PageState); ok {
			return state
		}
		return models.PageState{Path: "/home/articles", Params: models.NewFilters().ToQueryParams()}
	default:
		return models.PageState{Path: "/home"}
	}
}
