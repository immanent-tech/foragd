// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

var userCache = otter.Must(&otter.Options[string, models.User]{
	MaximumSize: 1000,
	ExpiryCalculator: otter.ExpiryAccessing[string, models.User](
		60 * time.Second,
	),
})

func loadUser(ctx context.Context, id string) (models.User, error) {
	switch resp, err := elastic.Search[*models.User](ctx, schema.UsersIndexRO(),
		query.Term("external_user_id", id, query.WithQueryName[*query.TermQuery]("get-user-by-external-id")),
		elastic.WithDocSorting(),
		elastic.WithTrackTotalHits(false),
		elastic.WithSize(1),
	); {
	case err != nil:
		return models.User{}, fmt.Errorf("%w: %w", otter.ErrNotFound, err)
	case len(resp.Results) == 0:
		return models.User{}, fmt.Errorf("%w: %w", otter.ErrNotFound, elastic.ErrNotFound)
	default:
		return *resp.Results[0], nil
	}
}

// GetUser retrieves the user doc with the given id.
func GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	user, err := elastic.GetDoc[models.UserID, *models.User](ctx, schema.UsersIndexRO(), id)
	if err != nil || user == nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// GetUserByExternalID will search for and return a user that matches the given external ID, if exists.
func GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	switch user, err := userCache.Get(ctx, externalID, otter.LoaderFunc[string, models.User](loadUser)); {
	case err != nil && !errors.Is(err, elastic.ErrNotFound):
		return nil, fmt.Errorf("find user by external id: %w", err)
	case errors.Is(err, elastic.ErrNotFound):
		return nil, fmt.Errorf("find user by external id: %w", models.ErrNotFound)
	default:
		return &user, nil
	}
}

// GetUserByEmail will retrieve a user by their email.
func GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	switch resp, err := elastic.Search[*models.User](
		ctx,
		schema.UsersIndexRO(),
		query.Term("email", email),
		elastic.WithDocSorting(),
		elastic.WithTrackTotalHits(false),
		elastic.WithSize(1),
	); {
	case err != nil:
		return nil, fmt.Errorf("search: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("search: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// GetUserBySubscriptionEmail will retrieve a user from their Foragd newsletter subscription email.
func GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*models.User, error) {
	switch resp, err := elastic.Search[*models.User](
		ctx,
		schema.UsersIndexRO(),
		query.Terms("settings.subscription_email", emails),
		elastic.WithDocSorting(),
		elastic.WithTrackTotalHits(false),
		elastic.WithSize(1),
	); {
	case err != nil:
		return nil, fmt.Errorf("find user by external id: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("find user by external id: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// UpdateUser will apply the given updates to the user.
func UpdateUser(ctx context.Context, user *models.User, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	if err := elastic.UpdateDoc(ctx, schema.UsersIndexRW(), user.GetID(), updates,
		elastic.WithRefresh(true),
		elastic.WithRetryOnConflict(client.DefaultRequestRetries),
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	slogctx.FromCtx(ctx).Info("User object updated.")
	// Invalidate any cached user data.
	userCache.Invalidate(user.GetExternalID())
	return nil
}
