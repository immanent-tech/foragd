// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/foragd/models"
)

var sessionCtx context.Context

// Make sure SessionStore implementation satisfies scs interfaces.
var (
	_ scs.IterableStore = (*Store)(nil)
	_ scs.Store         = (*Store)(nil)
)

// Store satisfies the session store interface for storing sessions in a custom backend.
type Store struct {
	data Datastore
}

// Datastore implements the methods needed to satisfy a session store.
type Datastore interface {
	GetSession(ctx context.Context, token string) (*models.UserSession, error)
	DeleteSession(ctx context.Context, token string) error
	UpdateSession(ctx context.Context, token string, data map[string]any) error
	FindAllSessions(ctx context.Context) ([]models.UserSession, error)
}

// NewSessionStore sets up a new session store for use by the server.
//
//nolint:fatcontext
func NewSessionStore(ctx context.Context, client Datastore) (*Store, error) {
	sessionCtx = ctx
	return &Store{
		data: client,
	}, nil
}

// Delete should remove the session token and corresponding data from the
// session store. If the token does not exist then Delete should be a no-op
// and return nil (not an error).
func (s *Store) Delete(token string) error {
	err := s.data.DeleteSession(sessionCtx, token)
	if err != nil {
		return fmt.Errorf("could not delete session: %w", err)
	}
	return nil
}

// Find should return the data for a session token from the store. If the
// session token is not found or is expired, the found return value should
// be false (and the err return value should be nil). Similarly, tampered
// or malformed tokens should result in a found return value of false and a
// nil err value. The err return value should be used for system errors only.
func (s *Store) Find(token string) ([]byte, bool, error) {
	session, err := s.data.GetSession(sessionCtx, token)
	if err != nil {
		return nil, false, fmt.Errorf("could not find a valid session: %w", err)
	}
	// Check for expired session.
	if session.Expiry.Before(time.Now().UTC()) {
		return nil, false, nil
	}

	return session.Data, true, nil
}

// Commit should add the session token and data to the store, with the given
// expiry time. If the session token already exists, then the data and
// expiry time should be overwritten.
func (s *Store) Commit(token string, b []byte, expiry time.Time) error {
	err := s.data.UpdateSession(sessionCtx, token, map[string]any{
		"token":      token,
		"data":       b,
		"expiry":     expiry,
		"updated_at": time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("could not commit session: %w", err)
	}

	return nil
}

// All should return a map containing data for all active sessions (i.e.
// sessions which have not expired). The map key should be the session
// token and the map value should be the session data. If no active
// sessions exist this should return an empty (not nil) map.
func (s *Store) All() (map[string][]byte, error) {
	data := make(map[string][]byte)
	sessions, err := s.data.FindAllSessions(sessionCtx)
	if err != nil {
		return nil, fmt.Errorf("unable to find active sessions: %w", err)
	}

	for _, session := range sessions {
		data[session.Token] = session.Data
	}

	return data, nil
}
