// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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

type sessionStore interface {
	NewSessionStorage() scs.Store
}

const (
	sessionLifetime = 24 * time.Hour
	sessionCookie   = "feedme"
)

const (
	profileSessionKey     = "tokens"
	preferencesSessionKey = "preferences"
	stateSessionKey       = "state"
	feedsSessionKey       = "feeds"
)

var (
	ErrDataNotFound = errors.New("data not found in session store")
	ErrInvalidData  = errors.New("invalid data in session")
)

var sessionManager *scs.SessionManager

func init() {
	gob.Register(models.Tokens{})
	gob.Register(models.UserPreferences{})
}

func NewSessionManager(store sessionStore) {
	// Set up the session manager.
	sessionManager = scs.New()
	sessionManager.Store = store.NewSessionStorage()
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
