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
	"github.com/immanent-tech/foragd/server/session/store"
)

const (
	sessionLifetime = 24 * time.Hour
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
		return value, fmt.Errorf("unable to restore session data as %T", value)
	}
	return value, nil
}

func Clear(ctx context.Context) error {
	if err := manager.Destroy(ctx); err != nil {
		return fmt.Errorf("clear session: %w", err)
	}
	return nil
}

func LoadAndSave(next http.Handler) http.Handler {
	return manager.LoadAndSave(next)
}
