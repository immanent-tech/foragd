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

// Manager manages saving, loading and manipulating a session.
type Manager struct {
	*scs.SessionManager
}

func init() {
	gob.Register(sessions.Session{})
	gob.Register(models.PageState{})
	gob.Register(models.List[models.PageState]{})
}

// NewSessionManager creates a new session manager.
func NewSessionManager(ctx context.Context, api *elastic.API, name string) (*Manager, error) {
	// Load the session store.
	sessionStore, err := store.NewSessionStore(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("failed to start session manager: %w", err)
	}
	mgr := &Manager{}
	// Set up the session manager.
	mgr.SessionManager = scs.New()
	mgr.Store = sessionStore
	mgr.Lifetime = sessionLifetime
	mgr.Cookie.Name = name
	mgr.Cookie.Secure = true
	mgr.Cookie.HttpOnly = true
	mgr.Cookie.SameSite = http.SameSiteLaxMode
	return mgr, nil
}
