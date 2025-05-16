// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
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
	GetSubscriptions(ctx context.Context) (models.Subscriptions, models.Pagination, error)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) error
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) error
	// Feeds methods:
	// GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, marks ...*models.MarkFeedItems) error
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

// FullRender renders a full page with the given title and options.
func FullRender(title string, pageOptions ...templates.PageOption) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if htmx.IsHTMX(req) {
			slogctx.FromCtx(req.Context()).Warn("Full render for HTMX request.")
		}
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
			if !htmx.IsHTMX(req) {
				slogctx.FromCtx(req.Context()).Warn("Partial render for non-HTMX request.")
			}
			resp, found := req.Context().Value(htmxRespCtxKey).(htmx.Response)
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No existing htmx response object, creating new one.")
				resp = htmx.NewResponse()
			}
			for template := range slices.Values(templates) {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
				}
			}
		})
}
