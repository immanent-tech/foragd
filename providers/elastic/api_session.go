// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"time"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

// GetSession retrieves session data with the given token.
func (a *API) GetSession(ctx context.Context, token string) (*models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	session, err := GetDoc[string, models.UserSession](ctx, a, index, token)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &session, nil
}

// DeleteSession removes the session data for the given token.
func (a *API) DeleteSession(ctx context.Context, token string) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	if err := DeleteDoc(ctx, a, index, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// UpdateSession updates the session data.
func (a *API) UpdateSession(ctx context.Context, token string, data map[string]any) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	if err := UpdateDoc(ctx, a, index,
		token,
		data,
		UpdateDocAsUpsert(),
	); err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

// FindAllSessions returns all active (non-expired) sessions.
func (a *API) FindAllSessions(ctx context.Context) ([]models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	sessions, err := SearchAll[models.UserSession](
		ctx,
		a,
		index,
		query.Since("expiry", time.Now().UTC()),
		defaultPaginationSize,
	)
	if err != nil {
		return nil, fmt.Errorf("find all sessions: %w", err)
	}
	return sessions, nil
}
