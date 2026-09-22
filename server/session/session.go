// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package session contains methods and objects for managing user sessions.
package session

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/foragd/server/session/store"
)

// sessionLifetime is the lifetime after which the session is expired. Set to the maximum session lifetime of Auth0
// sessions.
const sessionLifetime = 4320 * time.Minute

type Manager struct {
	*scs.SessionManager
}

var Load = sync.OnceValues(func() (*Manager, error) {
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("load store: %w", err)
	}
	// Set up the session manager.
	manager := scs.New()
	manager.Store = store
	manager.Lifetime = sessionLifetime
	manager.Cookie.Name = "foragd_session"
	manager.Cookie.Secure = true
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	return &Manager{
		SessionManager: manager,
	}, nil
})
