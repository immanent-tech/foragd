// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package session contains methods and objects for managing user sessions.
package session

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/markbates/goth/gothic"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/session/store"
)

const (
	sessionLifetime = 24 * time.Hour
	sessionName     = gothic.SessionName
)

const (
	subscriptionFiltersSessionKey = "subscription_filters"
	articleFiltersSessionKey      = "article_filters"
)

func init() {
	gob.Register(models.SubscriptionFilters{})
	gob.Register(models.ArticleFilters{})
	gob.Register(models.CommonFilters{})
}

var Manager *scs.SessionManager

// NewSessionManager creates a new session manager.
func NewSessionManager(ctx context.Context, api *elastic.API) error {
	// Load the session store.
	sessionStore, err := store.NewSessionStore(ctx, api)
	if err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}
	// Set up the session manager.
	Manager = scs.New()
	Manager.Store = sessionStore
	Manager.Lifetime = sessionLifetime
	Manager.Cookie.Name = sessionName
	Manager.Cookie.Secure = true
	Manager.Cookie.HttpOnly = true
	Manager.Cookie.SameSite = http.SameSiteLaxMode
	return nil
}

func FiltersToSession(ctx context.Context, filters any) {
	var key string
	switch filters.(type) {
	case *models.SubscriptionFilters:
		key = subscriptionFiltersSessionKey
	case *models.ArticleFilters:
		key = articleFiltersSessionKey
	default:
		return
	}
	Manager.Put(ctx, key, filters)
}

func SubscriptionFiltersFromSession(ctx context.Context) models.SubscriptionFilters {
	value, ok := Manager.Get(ctx, subscriptionFiltersSessionKey).(*models.SubscriptionFilters)
	if !ok {
		return models.NewSubscriptionFilters()
	}
	return *value
}

func ArticleFiltersFromSession(ctx context.Context) models.ArticleFilters {
	value, ok := Manager.Get(ctx, articleFiltersSessionKey).(*models.ArticleFilters)
	if !ok {
		return models.NewArticleFilters()
	}
	return *value
}
