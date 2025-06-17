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
	subscriptionRequestsCtxKey    contextKey = "subscriptionRequests"
	addSubscriptionsResultsCtxKey contextKey = "addSubscriptionsResults"

	htmxRespCtxKey contextKey = "htmxResponse"
	titleCtxKey    contextKey = "title"
	contentCtxKey  contextKey = "content"
	drawerCtxKey   contextKey = "drawer"
	pageCtxKey     contextKey = "page"
)

// Keys for objects stored within the session.
const (
	subscriptionsPageState = "subscriptions_state"
	articlesPageState      = "articles_state"
)

type contextKey string

// FeedsAPI contains methods for manipulating feed/item data.
type FeedsAPI interface {
	GetSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) (models.SubscriptionCustomisations, error)
	AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) (*bulk.Response, error)
	SearchSubscriptions(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.SubscriptionCustomisations, models.Pagination, error)
	GetFeeds(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error)
	SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error)
	ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, error)
	MultiSearch(ctx context.Context, feedsQuery, itemsQuery *query.MSearchOptions) (models.Feeds, models.Items, error)
}

// UserAPI contains methods for manipulating user data.
type UserAPI interface {
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	UpdateUser(ctx context.Context, id models.UserID, partialUpdate map[string]any) error
	UpdateSubscriptionCustomisation(ctx context.Context, id models.SubscriptionID, partialUpdate map[string]any) error
}

type UserBackendAPI interface {
	Create(ctx context.Context, details *models.UserSignupRequest) (string, error)
}

// BackendAPI contains the feed/user apis.
type BackendAPI interface {
	FeedsAPI
	UserAPI
}

// AuthAPI represents the API surface for interacting with the auth backend.
type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) models.UserID
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

// RenderContentPage will render a full page (i.e. non-HTMX) response.
func RenderPage() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			// Get main content.
			page, ok := req.Context().Value(pageCtxKey).(templ.Component)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("Invalid or missing page content.")
				http.Error(res, "Invalid page content.", http.StatusInternalServerError)
				return
			}
			if err := page.Render(req.Context(), res); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			}
		})
}

// RenderContentPage will render a full page (i.e. non-HTMX) response.
func RenderContentPage() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			// Get main content.
			mainContent, ok := req.Context().Value(contentCtxKey).(templ.Component)
			if !ok {
				// If there is no content, use the empty content template.
				mainContent = views.EmptyContent()
			}
			// Wrap main content.
			drawerContent := templ.Join(views.Header(), partials.Content(mainContent), partials.Footer())
			// Get drawer side content.
			drawerSideContent, ok := req.Context().Value(drawerCtxKey).(templ.Component)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("Invalid content.")
				http.Error(res, "Invalid content.", http.StatusInternalServerError)
				return
			}
			// Wrap drawer side content.
			drawerSideContent = partials.DrawerMenu(
				partials.MenuItemTitle("Navigation"),
				views.DrawerHomeLink(),
				views.DrawerSubscriptionList(drawerSideContent),
			)
			// Get page title.
			title, ok := req.Context().Value(titleCtxKey).(string)
			if !ok {
				title = "Go Feed Me"
			}
			// Render page.
			page := templates.NewPage(title,
				partials.Drawer(drawerContent, drawerSideContent),
			).Show()
			if err := page.Render(req.Context(), res); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			}
		})
}

// RenderContentPartials will render individual content updates (i.e., HTMX response).
func RenderContentPartials() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			var partials []templ.Component
			// Add any content updates.
			if content, ok := req.Context().Value(contentCtxKey).(templ.Component); ok {
				partials = append(partials, content)
			}
			// Add any drawer side-bar updates.
			if content, ok := req.Context().Value(drawerCtxKey).(templ.Component); ok {
				partials = append(partials, content)
			}
			// Add any page title updates.
			if title, ok := req.Context().Value(titleCtxKey).(string); ok {
				partials = append(partials, templates.SetPageTitle(title))
			}
			// Get any existing htmx response writer.
			resp, ok := req.Context().Value(htmxRespCtxKey).(htmx.Response)
			if !ok {
				resp = htmx.NewResponse()
			}
			// Render as a template.
			if err := resp.RenderTempl(req.Context(), res, templ.Join(partials...)); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
			}
		})
}

// ProcessResponse handles appropriate display and logging of a models.Response object.
func ProcessResponse(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.Any("error", resp.InternalError))
	// Display a notification if a user message is set.
	if resp.UserMessage != nil {
		if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.ShowNotification(resp.UserMessage)); err != nil {
			http.Error(res, "Internal server error.", http.StatusInternalServerError)
		}
	}
	// Write the status code.
	res.WriteHeader(resp.StatusCode)
}
