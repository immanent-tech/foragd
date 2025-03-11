// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

var sessionCtx context.Context

// Make sure SessionStore implementation satisfies scs interfaces.
var (
	_ scs.IterableStore = (*Store)(nil)
	_ scs.Store         = (*Store)(nil)
)

var (
	ErrInitStoreFailed     = errors.New("could not initialize session store")
	ErrDeleteSessionFailed = errors.New("delete session failed")
	ErrFindSessionFailed   = errors.New("could not find session")
	ErrCommitSessionFailed = errors.New("could not commit session")
)

type Store struct {
	logger *slog.Logger
	client *elastic.Client
	index  string
}

// DeleteCtx should remove the session token and corresponding data from the
// session store. If the token does not exist then Delete should be a no-op
// and return nil (not an error).
func (s *Store) Delete(token string) error {
	_, err := elastic.NewDocDeleteRequest(s.client.GetAPI(),
		s.index,
		token,
		refresh.True,
	).Do(sessionCtx)
	if err != nil {
		return errors.Join(ErrDeleteSessionFailed, err)
	}

	return nil
}

// Find should return the data for a session token from the store. If the
// session token is not found or is expired, the found return value should
// be false (and the err return value should be nil). Similarly, tampered
// or malformed tokens should result in a found return value of false and a
// nil err value. The err return value should be used for system errors only.
func (s *Store) Find(token string) ([]byte, bool, error) {
	resp, err := elastic.NewGetRequest(s.client.GetAPI(),
		s.index,
		token,
	).Do(sessionCtx)
	if err != nil {
		return nil, false, errors.Join(ErrFindSessionFailed, err)
	}
	// Check for session found.
	if !resp.Found {
		return nil, false, nil
	}
	// Extract (and validate) session data.
	session, err := elastic.ExtractSource[models.UserSession](resp.Source_)
	if err != nil {
		return nil, false, errors.Join(ErrFindSessionFailed, err)
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
	session := &models.UserSession{
		Token:  token,
		Data:   b,
		Expiry: expiry,
	}

	s.logger.Debug("Committing new session.",
		slog.String("token", session.Token),
		slog.Time("expiry", session.Expiry))

	_, err := elastic.NewDocUpdateRequest(s.client.GetAPI(),
		s.index,
		token,
		elastic.WithDocUpdate(session, true),
		elastic.WithForcedRefresh(),
	).Do(sessionCtx)
	if err != nil {
		return errors.Join(ErrCommitSessionFailed, err)
	}

	return nil
}

// All should return a map containing data for all active sessions (i.e.
// sessions which have not expired). The map key should be the session
// token and the map value should be the session data. If no active
// sessions exist this should return an empty (not nil) map.
func (s *Store) All() (map[string][]byte, error) {
	searchSize := 1000
	pagination := make([]types.FieldValue, 0)
	data := make(map[string][]byte)

	// Loop until we've paginated through all results.
	for {
		var (
			sessions []models.UserSession
			warnings error
		)

		resp, err := elastic.NewSearchRequest(s.client.GetAPI(),
			elastic.WithSearchIndex(s.index),
			elastic.WithSearchQueryOptions(elastic.QuerySince("expiry", time.Now().UTC())),
			elastic.WithSearchSize(searchSize),
			elastic.WithSearchAfter(pagination),
		).Do(sessionCtx)
		if err != nil {
			return nil, errors.Join(elastic.ErrSearchFailed, err)
		}
		// Stop if there are no hits
		if len(resp.Hits.Hits) == 0 {
			return nil, nil
		}
		// Loop through this set of results.
		sessions, pagination, warnings = elastic.ExtractSourceFromHits[models.UserSession](resp.Hits.Hits)
		if warnings != nil {
			s.logger.Warn("Could not extract some session data.",
				slog.Any("warnings", warnings))
		}

		for _, session := range sessions {
			data[session.Token] = session.Data
		}

		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resp.Hits.Hits) < searchSize {
			break
		}
	}

	return data, nil
}

func NewSessionStore(ctx context.Context, client *elastic.Client) (*Store, error) {
	sessionCtx = ctx

	return &Store{
		logger: logging.FromContext(ctx).WithGroup("session"),
		client: client,
		index:  schema.SessionsPrefix,
	}, nil
}
