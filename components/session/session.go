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
	"github.com/gorilla/sessions"

	"github.com/joshuar/go-feed-me/components/session/store"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
)

const (
	sessionLifetime            = 24 * time.Hour
	apiCtxKey       contextKey = "session"
)

type contextKey string

var Manager *scs.SessionManager

func init() {
	gob.Register(sessions.Session{})
	gob.Register(models.PageState{})
	gob.Register(models.List[models.PageState]{})
}

// NewSessionManager creates a new session manager.
func NewSessionManager(ctx context.Context, api *elastic.API, name string) error {
	// Load the session store.
	sessionStore, err := store.NewSessionStore(ctx, api)
	if err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}
	// Set up the session manager.
	Manager = scs.New()
	Manager.Store = sessionStore
	Manager.Lifetime = sessionLifetime
	Manager.Cookie.Name = name
	Manager.Cookie.Secure = true
	Manager.Cookie.HttpOnly = true
	Manager.Cookie.SameSite = http.SameSiteLaxMode
	return nil
}
