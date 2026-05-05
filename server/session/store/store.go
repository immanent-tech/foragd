// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

const (
	// defaultRequestTimeout is the maximum time a background action can run before its context is cancelled.
	defaultRequestTimeout = 5 * time.Second
)

// Make sure SessionStore implementation satisfies scs interfaces.
var (
	_ scs.IterableCtxStore = (*Store)(nil)
	_ scs.CtxStore         = (*Store)(nil)
)

// Store satisfies the session store interface for storing sessions in a custom backend.
type Store struct {
	cache *otter.Cache[string, []byte]
}

// NewSessionStore sets up a new session store for use by the server.
func NewSessionStore(sessionLifetime time.Duration) (*Store, error) {
	if err := config.Init(); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	cache, err := otter.New(&otter.Options[string, []byte]{
		MaximumSize:      10_000,
		ExpiryCalculator: otter.ExpiryWriting[string, []byte](sessionLifetime),
	})
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &Store{cache: cache}, nil
}

// DeleteCtx should remove the session token and corresponding data from the
// session store. If the token does not exist then Delete should be a no-op
// and return nil (not an error).
func (s *Store) DeleteCtx(ctx context.Context, token string) error {
	s.cache.Invalidate(token)
	if err := elastic.DeleteDoc(ctx, schema.SessionsIndexRW(), token); err != nil {
		return fmt.Errorf("could not delete session: %w", err)
	}
	return nil
}

func (s *Store) Delete(token string) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancelFunc()
	return s.DeleteCtx(ctx, token)
}

// FindCtx should return the data for a session token from the store. If the
// session token is not found or is expired, the found return value should
// be false (and the err return value should be nil). Similarly, tampered
// or malformed tokens should result in a found return value of false and a
// nil err value. The err return value should be used for system errors only.
func (s *Store) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	if val, ok := s.cache.GetIfPresent(token); ok {
		slogctx.FromCtx(ctx).Debug("Session cache hit!")
		return val, true, nil // cache hit
	}
	session, err := elastic.GetDoc[string, models.UserSession](ctx, schema.SessionsIndexRO(), token)
	if err != nil {
		return nil, false, fmt.Errorf("could not find a valid session: %w", err)
	}

	// Check for expired session.
	if session.Expiry.Before(time.Now().UTC()) {
		return nil, false, nil
	}

	return session.Data, true, nil
}

func (s *Store) Find(token string) ([]byte, bool, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancelFunc()
	return s.FindCtx(ctx, token)
}

// CommitCtx should add the session token and data to the store, with the given
// expiry time. If the session token already exists, then the data and
// expiry time should be overwritten.
func (s *Store) CommitCtx(ctx context.Context, token string, data []byte, expiry time.Time) error {
	if err := elastic.UpdateDoc(ctx, schema.SessionsIndexRW(),
		token,
		map[string]any{
			"token":      token,
			"data":       data,
			"expiry":     expiry,
			"updated_at": time.Now().UTC(),
		},
		elastic.WithDocAsUpsert(true),
		elastic.WithRefresh(true),
	); err != nil {
		return fmt.Errorf("could not commit session: %w", err)
	}
	if time.Until(expiry) <= 0 {
		s.cache.Invalidate(token)
		return nil
	}
	s.cache.Set(token, data)

	return nil
}

func (s *Store) Commit(token string, b []byte, expiry time.Time) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancelFunc()
	return s.CommitCtx(ctx, token, b, expiry)
}

// AllCtx should return a map containing data for all active sessions (i.e.
// sessions which have not expired). The map key should be the session
// token and the map value should be the session data. If no active
// sessions exist this should return an empty (not nil) map.
func (s *Store) AllCtx(ctx context.Context) (map[string][]byte, error) {
	const defaultPaginationSize = 5000
	sessions, err := elastic.SearchAll[models.UserSession](
		ctx,
		schema.SessionsIndexRO(),
		query.Since("expiry", time.Now().UTC()),
		defaultPaginationSize,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to find active sessions: %w", err)
	}

	data := make(map[string][]byte, len(sessions))

	for _, session := range sessions {
		data[session.Token] = session.Data
	}

	return data, nil
}

func (s *Store) All() (map[string][]byte, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancelFunc()
	return s.AllCtx(ctx)
}
