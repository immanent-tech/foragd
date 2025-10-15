// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package session contains methods and objects for managing user sessions.
package session

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/session/store"
)

const (
	sessionLifetime               = 24 * time.Hour
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
	Manager.Cookie.Name = strings.ToLower(config.AppName) + "_session"
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

// SaveToSession saves the given object to the session storage with the given key.
func SaveToSession[T any](ctx context.Context, key string, obj T) {
	Manager.Put(ctx, key, obj)
}

// RestoreFromSession retrieves an object from the session storage with the given key. If the object cannot be
// retrieved, then the defaultFunc is used to generate a new default value.
func RestoreFromSession[T any](ctx context.Context, key string, defaultFunc func() T) T {
	value, ok := Manager.Get(ctx, articleFiltersSessionKey).(T)
	if !ok {
		return defaultFunc()
	}
	return value
}
