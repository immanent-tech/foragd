// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"encoding/gob"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
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
	feedsCtxKey                contextKey = "feeds"

	headerContentCtxKey contextKey = "headerContent"
	footerContentCtxKey contextKey = "footerContent"
	drawerContentCtxKey contextKey = "drawerContent"
	contentCtxKey       contextKey = "content"
)

// Keys for objects stored within the session.
const (
	subscriptionsPageState = "subscriptions_state"
	articlesPageState      = "articles_state"
)

type contextKey string

type FeedsAPI interface {
	GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error)
	GetFeedsByID(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error)
	GetTopItemCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, *models.Response)
	GetSubscriptionUnreadCounts(ctx context.Context, subscriptions models.Subscriptions) error
	GetArticlesByID(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error)
	ItemsSearch(ctx context.Context, query query.Option, filters models.Filters, pagination models.Pagination) (*search.Response, error)
	ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, error)
}

type UserAPI interface {
	UpdateUser(ctx context.Context, id models.UserID, partialUpdate map[string]any) *models.Response
}

// DataAPI represents the API surface for interacting with the database/datastore backend.
type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	// Subscription methods:
	GetSubscriptionsByID(ctx context.Context, filters models.Filters, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Subscriptions, models.Pagination, *models.Response)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) *models.Response
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) *models.Response
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) *models.Response
	// Feeds methods:
	// GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetFeedsByID(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error)
	GetArticle(ctx context.Context, itemID models.ItemID) (*models.Article, bool, *models.Response)
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

// RouteLogger decorates the logger in the request context with routing information.
func RouteLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := slogctx.With(req.Context(),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("method", req.Method),
		)
		ctx = slogctx.With(ctx, slog.Group("req", slog.String("id", middleware.GetReqID(ctx))))
		slogctx.FromCtx(ctx).Debug("Processing route.", slog.String("url", req.URL.String()))

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// RenderPage will render a full page (i.e. non-HTMX) response.
func RenderPage(title string) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			header, ok := req.Context().Value(headerContentCtxKey).(templ.Component)
			if !ok || header == nil {
				header = partials.Header()
			}
			footer, ok := req.Context().Value(footerContentCtxKey).(templ.Component)
			if !ok || header == nil {
				footer = partials.Footer()
			}
			drawer, ok := req.Context().Value(drawerContentCtxKey).(templ.Component)
			if !ok || drawer == nil {
				drawer = partials.Drawer()
			}
			content, ok := req.Context().Value(contentCtxKey).(templ.Component)
			if !ok {
				slogctx.FromCtx(req.Context()).Error("Invalid content.")
				http.Error(res, "Invalid content.", http.StatusInternalServerError)
				return
			}
			page := views.FullPage(title, header, footer, drawer, content)
			if err := page.Render(req.Context(), res); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			}
		})
}

// RenderPartials will render individual content updates (i.e., HTMX response).
func RenderPartials(title string) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			var partials []templ.Component
			if content, ok := req.Context().Value(contentCtxKey).(templ.Component); ok {
				partials = append(partials, content)
			}
			if header, ok := req.Context().Value(headerContentCtxKey).(templ.Component); ok {
				partials = append(partials, header)
			}
			if footer, ok := req.Context().Value(footerContentCtxKey).(templ.Component); ok {
				partials = append(partials, footer)
			}
			if drawer, ok := req.Context().Value(drawerContentCtxKey).(templ.Component); ok {
				partials = append(partials, drawer)
			}
			if title != "" {
				partials = append(partials, templates.SetPageTitle(title))
			}
			resp := htmx.NewResponse()
			for template := range slices.Values(partials) {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
				}
			}
		})
}

// ProcessResponse handles appropriate display and logging of a models.Response object.
func ProcessResponse(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.String("error", resp.String()))
	// Display a notification if a user message is set.
	if resp.UserMessage != nil {
		if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.ShowNotification(resp.UserMessage)); err != nil {
			http.Error(res, "Internal server error.", http.StatusInternalServerError)
		}
	}
	// Write the status code.
	res.WriteHeader(resp.StatusCode)
}
