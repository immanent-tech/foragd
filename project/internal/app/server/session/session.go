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
	profileSessionKey          = "tokens"
	preferencesSessionKey      = "preferences"
	stateSessionKey            = "state"
	listFeedsFiltersSessionKey = "feeds_state"
	listItemsFiltersSessionKey = "items_state"
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
	gob.Register(models.APIFilters{})
}

func NewSessionManager(store scs.Store) {
	// Set up the session manager.
	session.SessionManager = scs.New()
	session.Store = store
	session.Lifetime = sessionLifetime
	session.Cookie.Name = sessionCookie
	session.Cookie.Secure = true
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

func SaveListFeedsFilters(ctx context.Context, filters *models.APIFilters) {
	if filters == nil {
		session.logger.Warn("Cannot store feed filters: filters is nil.")
		return
	}

	// spew.Dump(filters)

	session.Put(ctx, listFeedsFiltersSessionKey, *filters)
}

func LoadListFeedsFilters(ctx context.Context) (models.APIFilters, error) {
	data := session.Get(ctx, listFeedsFiltersSessionKey)
	filters, ok := data.(models.APIFilters)

	switch {
	case data == nil:
		return models.APIFilters{}, ErrDataNotFound
	case ok:
		return filters, nil
	default:
		return models.APIFilters{}, ErrInvalidData
	}
}

func SaveListItemsFilters(ctx context.Context, filters *models.APIFilters) {
	if filters == nil {
		session.logger.Warn("Cannot store items filters: filters is nil.")
		return
	}

	session.Put(ctx, listItemsFiltersSessionKey, filters)
}

func LoadListItemsFilters(ctx context.Context) (models.APIFilters, error) {
	data := session.Get(ctx, listItemsFiltersSessionKey)
	filters, ok := data.(models.APIFilters)

	switch {
	case data == nil:
		return models.APIFilters{}, ErrDataNotFound
	case ok:
		return filters, nil
	default:
		return models.APIFilters{}, ErrInvalidData
	}
}
