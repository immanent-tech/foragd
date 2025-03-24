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

type manager struct {
	*scs.SessionManager
	logger *slog.Logger
}

var session = &manager{logger: slog.Default()}

func init() {
	gob.Register(models.Tokens{})
	gob.Register(models.Filters{})
}

func NewSessionManager(store scs.Store) {
	// Set up the session manager.
	session.SessionManager = scs.New()
	session.Store = store
	session.Lifetime = sessionLifetime
	session.Cookie.Name = sessionCookie
	session.Cookie.Secure = true
	session.Cookie.HttpOnly = true
	session.Cookie.SameSite = http.SameSiteLaxMode
}

func ClearSession(ctx context.Context) error {
	if err := session.Clear(ctx); err != nil {
		return fmt.Errorf("unable to clear session data: %w", err)
	}

	if err := session.Destroy(ctx); err != nil {
		return fmt.Errorf("unable to remove session: %w", err)
	}

	return nil
}

func LoadAndSave() func(next http.Handler) http.Handler {
	return session.LoadAndSave
}

func StoreFeedFilters(ctx context.Context, filters *models.Filters) {
	session.Put(ctx, feedFiltersKey, *filters)
}

func GetFeedFilters(ctx context.Context) (*models.Filters, error) {
	filters, ok := session.Get(ctx, feedFiltersKey).(models.Filters)
	if !ok {
		return nil, models.WrapError(ErrDataNotFound, "session", "no feed filters in session")
	}
	return &filters, nil
}

func StoreItemFilters(ctx context.Context, filters *models.Filters) {
	session.Put(ctx, itemFiltersKey, *filters)
}

func GetItemFilters(ctx context.Context) (*models.Filters, error) {
	filters, ok := session.Get(ctx, itemFiltersKey).(models.Filters)
	if !ok {
		return nil, models.WrapError(ErrDataNotFound, "session", "no item filters in session")
	}
	return &filters, nil
}
