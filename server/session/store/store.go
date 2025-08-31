// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/providers/elastic/schema"
)

var sessionCtx context.Context

// Make sure SessionStore implementation satisfies scs interfaces.
var (
	_ scs.IterableStore = (*Store)(nil)
	_ scs.Store         = (*Store)(nil)
)

type Store struct {
	client *elastic.API
	index  string
}

// NewSessionStore sets up a new session store for use by the server.
func NewSessionStore(ctx context.Context, client *elastic.API) (*Store, error) {
	sessionCtx = ctx
	return &Store{
		client: client,
		index:  schema.SessionsSchemaPrefix,
	}, nil
}

// Delete should remove the session token and corresponding data from the
// session store. If the token does not exist then Delete should be a no-op
// and return nil (not an error).
func (s *Store) Delete(token string) error {
	err := elastic.DeleteDoc(sessionCtx, s.client.GetAPI(), s.index, token)
	if err != nil {
		return fmt.Errorf("delete session failed: %w", err)
	}

	return nil
}

// Find should return the data for a session token from the store. If the
// session token is not found or is expired, the found return value should
// be false (and the err return value should be nil). Similarly, tampered
// or malformed tokens should result in a found return value of false and a
// nil err value. The err return value should be used for system errors only.
func (s *Store) Find(token string) ([]byte, bool, error) {
	session, err := elastic.GetDoc[string, models.UserSession](sessionCtx, s.client.GetAPI(), s.index, token)
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
	err := elastic.UpdateDoc(sessionCtx, s.client.GetAPI(), s.index,
		token,
		map[string]any{
			"token":      token,
			"data":       b,
			"expiry":     expiry,
			"updated_at": time.Now().UTC(),
		},
		elastic.UpdateDocAsUpsert(),
	)
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

	sessions, err := elastic.SearchAll[models.UserSession](sessionCtx, s.client.GetAPI(), s.index, query.Since("expiry", time.Now().UTC()), 1000)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve active sessions: %w", err)
	}

	for _, session := range sessions {
		data[session.Token] = session.Data
	}

	return data, nil
}
