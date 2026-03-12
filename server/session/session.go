// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package session contains methods and objects for managing user sessions.
package session

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/session/store"
)

const (
	sessionLifetime = 24 * time.Hour

	listSubscriptionFiltersKey = "listSubscriptionFilters"
	listArticleFiltersKey      = "listArticleFilters"
)

var manager *scs.SessionManager

// NewSessionManager creates a new session manager.
func NewSessionManager() error {
	// Load the session store.
	sessionStore, err := store.NewSessionStore()
	if err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}
	// Set up the session manager.
	manager = scs.New()
	manager.Store = sessionStore
	manager.Lifetime = sessionLifetime
	manager.Cookie.Name = strings.ToLower(config.AppName) + "_session"
	manager.Cookie.Secure = true
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	return nil
}

// Save saves the given object to the session storage with the given key.
func Save[T any](ctx context.Context, key string, obj T) {
	manager.Put(ctx, key, obj)
}

// Restore retrieves an object from the session storage with the given key. If the object cannot be
// retrieved, then the defaultFunc is used to generate a new default value.
func Restore[T any](ctx context.Context, key string) (T, error) {
	value, ok := manager.Get(ctx, key).(T)
	if !ok {
		return value, fmt.Errorf("unable to restore session data as %T (data %v)", value, value)
	}
	return value, nil
}

// Renew updates the session data to have a new session token while retaining the current session data. The session
// lifetime is also reset and the session data status will be set to Modified. The old session token and accompanying
// data are deleted from the session store.
func Renew(ctx context.Context) error {
	if err := manager.RenewToken(ctx); err != nil {
		return fmt.Errorf("unable to renew token: %w", err)
	}
	return nil
}

// Clear deletes the session data from the session store and sets the session status to Destroyed. Any further
// operations in the same request cycle will result in a new session being created.
func Clear(ctx context.Context) error {
	if err := manager.Destroy(ctx); err != nil {
		return fmt.Errorf("clear session: %w", err)
	}
	return nil
}

// LoadAndSave middleware handles loading the session data for a request and saving any modifications.
func LoadAndSave(next http.Handler) http.Handler {
	return manager.LoadAndSave(next)
}

func GetListSubscriptionFiltersFromSession(ctx context.Context) *models.ListFilters {
	restored, err := Restore[models.ListFilters](ctx, listSubscriptionFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = models.NewListDisplayFilters()
	}

	return &restored
}

func StoreListSubscriptionFiltersInSession(ctx context.Context, filters models.ListFilters) {
	Save(ctx, listSubscriptionFiltersKey, filters)
}

func GetListArticleFiltersFromSession(ctx context.Context) *models.ListFilters {
	restored, err := Restore[models.ListFilters](ctx, listArticleFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = models.NewListDisplayFilters()
	}

	return &restored
}

func StoreListArticleFiltersInSession(ctx context.Context, filters models.ListFilters) {
	Save(ctx, listArticleFiltersKey, filters)
}
