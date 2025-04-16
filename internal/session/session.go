// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/auth"
	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	sessionLifetime = 24 * time.Hour
	sessionCookie   = "feedme"
)

const (
	profileSessionKey     = "tokens"
	preferencesSessionKey = "preferences"
	stateSessionKey       = "state"
	routeStateSessionKey  = "route_state"
	feedFiltersKey        = "feed_filters"
	itemFiltersKey        = "item_filters"
)

var (
	ErrDataNotFound = errors.New("data not found in session store")
	ErrInvalidData  = errors.New("invalid data in session")
)

type Manager struct {
	*scs.SessionManager
}

var mgr = &Manager{}

func init() {
	gob.Register(models.Filters{})
	gob.Register(sessions.Session{})
	gob.Register(auth.UserAuth{})
}

func NewSessionManager(ctx context.Context, store scs.Store) {
	// Set up the session manager.
	mgr.SessionManager = scs.New()
	mgr.Store = store
	mgr.Lifetime = sessionLifetime
	mgr.Cookie.Name = sessionCookie
	mgr.Cookie.Secure = true
	mgr.Cookie.HttpOnly = true
	mgr.Cookie.SameSite = http.SameSiteLaxMode
}

func GetSessionManager() *Manager {
	return mgr
}

func ClearSession(ctx context.Context) error {
	if err := mgr.Clear(ctx); err != nil {
		return fmt.Errorf("unable to clear session data: %w", err)
	}

	if err := mgr.Destroy(ctx); err != nil {
		return fmt.Errorf("unable to remove session: %w", err)
	}

	return nil
}

func LoadAndSave() func(next http.Handler) http.Handler {
	return mgr.LoadAndSave
}

func StoreFeedFilters(ctx context.Context, filters *models.Filters) {
	mgr.Put(ctx, feedFiltersKey, filters)
}

func GetFeedFilters(ctx context.Context) models.Filters {
	filters, ok := mgr.Get(ctx, feedFiltersKey).(models.Filters)
	if !ok {
		logger(ctx).Warn("No feed filters in session, using default filters.")
		return *models.NewFilters()
	}
	return filters
}

func StoreItemFilters(ctx context.Context, filters *models.Filters) {
	mgr.Put(ctx, itemFiltersKey, filters)
}

func GetItemFilters(ctx context.Context) models.Filters {
	filters, ok := mgr.Get(ctx, itemFiltersKey).(models.Filters)
	if !ok {
		logger(ctx).Warn("No item filters in session, using default filters.")
		return *models.NewFilters()
	}
	return filters
}

func logger(ctx context.Context) *slog.Logger {
	logger := slogctx.FromCtx(ctx)
	if id := middleware.GetReqID(ctx); id != "" {
		logger = logger.With(slog.String("id", id))
	}
	return logger.WithGroup("session")
}
