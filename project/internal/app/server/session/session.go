// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
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

var sessionManager *scs.SessionManager

func init() {
	gob.Register(models.Tokens{})
	gob.Register(models.APIFilters{})
}

func NewSessionManager(store scs.Store) {
	// Set up the session manager.
	sessionManager = scs.New()
	sessionManager.Store = store
	sessionManager.Lifetime = sessionLifetime
	sessionManager.Cookie.Name = sessionCookie
	sessionManager.Cookie.Secure = true
}

func ClearSession(ctx context.Context) error {
	if err := sessionManager.Clear(ctx); err != nil {
		return fmt.Errorf("unable to clear session data: %w", err)
	}

	if err := sessionManager.Destroy(ctx); err != nil {
		return fmt.Errorf("unable to remove session: %w", err)
	}

	return nil
}

func LoadAndSave() func(next http.Handler) http.Handler {
	return sessionManager.LoadAndSave
}

func SaveListFeedsFilters(ctx context.Context, filters models.APIFilters) {
	sessionManager.Put(ctx, listFeedsFiltersSessionKey, filters)
}

func LoadListFeedsFilters(ctx context.Context) (models.APIFilters, error) {
	data := sessionManager.Get(ctx, listFeedsFiltersSessionKey)
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

func SaveListItemsFilters(ctx context.Context, filters models.APIFilters) {
	sessionManager.Put(ctx, listItemsFiltersSessionKey, filters)
}

func LoadListItemsFilters(ctx context.Context) (models.APIFilters, error) {
	data := sessionManager.Get(ctx, listItemsFiltersSessionKey)
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
