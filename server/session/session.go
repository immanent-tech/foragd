// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package session contains methods and objects for managing user sessions.
package session

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/session/store"
)

const (
	sessionLifetime = 4320 * time.Minute

	listSubscriptionFiltersKey = "listSubscriptionFilters"
	listArticleFiltersKey      = "listArticleFilters"
)

var Load = sync.OnceValue(func() *scs.SessionManager {
	// Set up the session manager.
	manager := scs.New()
	manager.Store = store.New()
	manager.Lifetime = sessionLifetime
	manager.Cookie.Name = strings.ToLower(config.GetAppName()) + "_session"
	manager.Cookie.Secure = true
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	return manager
})

// Save saves the given object to the session storage with the given key.
func Save[T any](ctx context.Context, key string, obj T) error {
	manager := Load()
	manager.Put(ctx, key, obj)
	return nil
}

func SaveAndCommit[T any](ctx context.Context, key string, obj T) error {
	manager := Load()
	manager.Put(ctx, key, obj)
	if _, _, err := manager.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Restore retrieves an object from the session storage with the given key. If there is an error retrieving the object
// as the given type a non-nill error containing details will be returned.
func Restore[T any](ctx context.Context, key string) (T, error) {
	manager := Load()
	value, ok := manager.Get(ctx, key).(T)
	if !ok {
		return value, fmt.Errorf("unable to restore %s session data as %T (data %v)", key, value, value)
	}
	return value, nil
}

// Renew updates the session data to have a new session token while retaining the current session data. The session
// lifetime is also reset and the session data status will be set to Modified. The old session token and accompanying
// data are deleted from the session store.
func Renew(ctx context.Context) error {
	manager := Load()
	if err := manager.RenewToken(ctx); err != nil {
		return fmt.Errorf("unable to renew token: %w", err)
	}
	return nil
}

// Remove will remove the value with the given key from the session.
func Remove(ctx context.Context, key string) error {
	manager := Load()
	manager.Remove(ctx, key)
	return nil
}

// Clear deletes the session data from the session store and sets the session status to Destroyed. Any further
// operations in the same request cycle will result in a new session being created.
func Clear(ctx context.Context) error {
	manager := Load()
	if err := manager.Destroy(ctx); err != nil {
		return fmt.Errorf("clear session: %w", err)
	}
	return nil
}

func GetListSubscriptionFiltersFromSession(ctx context.Context) *models.ListFilters {
	restored, err := Restore[models.ListFilters](ctx, listSubscriptionFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = models.NewListDisplayFilters()
	}

	return &restored
}

func StoreListSubscriptionFiltersInSession(ctx context.Context, filters models.ListFilters) error {
	return Save(ctx, listSubscriptionFiltersKey, filters)
}

func GetListArticleFiltersFromSession(ctx context.Context) *models.ListFilters {
	restored, err := Restore[models.ListFilters](ctx, listArticleFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = models.NewListDisplayFilters()
	}

	return &restored
}

func StoreListArticleFiltersInSession(ctx context.Context, filters models.ListFilters) error {
	return Save(ctx, listArticleFiltersKey, filters)
}
