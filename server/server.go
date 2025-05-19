// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/auth"
	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/components/session"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/auth0"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

const (
	RequestIDKey = "request_id"
)

type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context, userID models.UserID) (*models.User, error)
	// Subscription methods:
	GetSubscription(ctx context.Context, subscriptionID models.SubscriptionID) (*models.Subscription, error)
	GetSubscriptions(ctx context.Context, pagination models.Pagination) (models.Subscriptions, models.Pagination, error)
	MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) error
	RemoveSubscriptions(ctx context.Context, subscriptionIDs ...models.SubscriptionID) error
	// Feeds methods:
	FeedsSearchAll(ctx context.Context, queries ...query.Option) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context, pagination models.Pagination) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, marks ...*models.MarkFeedItems) error
	GetTopItemCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, error)
}

type API struct {
	user    *auth0.UserAPI
	elastic DataAPI
	auth    *auth.Authenticator
	session *session.Manager
}

type Server struct {
	API *API
	Log *slog.Logger
}

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

var ErrStartServer = errors.New("start server failed")

func NewServer(ctx context.Context) (Server, error) {
	var svr Server
	// Load the server config.
	if err := config.Load(serverConfigPrefix, serverConfigEnvPrefix, ServerConfig); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// If no secret is set, create a new secret.
	if ServerConfig.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return svr, fmt.Errorf("%w: %w", ErrLoadConfig, err)
		}

		ServerConfig.Secret = secret
	}
	// Set up the logger
	svr.Log = slogctx.FromCtx(ctx)
	// Load the auth0UserAPI backend.
	auth0UserAPI, err := auth0.NewUserAPI(ctx)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Set up the session manager.
	sessionAPI, err := session.NewSessionManager(ctx, elasticAPI, auth.SessionName)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Set up authentication manager.
	authAPI, err := auth.NewAuthenticator(ctx, sessionAPI)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}

	// Add the API to the environment.
	svr.API = &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		auth:    authAPI,
		session: sessionAPI,
	}

	return svr, nil
}

func backendErrorMsg(err error) *models.Message {
	return models.NewMessage(
		"A backend error occurred.",
		models.MessageStatusError,
		models.WithError(err))
}
