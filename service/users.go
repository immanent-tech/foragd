/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/create"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/delete"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/update"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"

	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel/codes"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// UserService holds a cache and backend connection for handling [models.User] objects.
type UserService struct {
	*otter.Cache[string, models.User]

	store  *ElasticService
	loader otter.LoaderFunc[string, models.User]
}

// LoadUserService loads a service that can manipulate user objects in the backend store.
var LoadUserService = sync.OnceValues(func() (*UserService, error) {
	svc, err := LoadElasticService()
	if err != nil {
		return nil, fmt.Errorf("load elastic service: %w", err)
	}
	return &UserService{
		Cache: otter.Must(&otter.Options[string, models.User]{
			MaximumSize: 100,
			ExpiryCalculator: otter.ExpiryAccessing[string, models.User](
				60 * time.Second,
			),
		}),
		store: svc,
		loader: otter.LoaderFunc[string, models.User](
			func(ctx context.Context, id string) (models.User, error) {
				index := ctx.Value("users_index").(string)
				switch resp, err := elastic.Search[*models.User](ctx,
					index,
					elastic.WithQuery[*search.Search](
						query.Term("external_user_id", id, query.WithQueryName[*query.TermQuery]("get-user-by-external-id")),
					),
					elastic.WithDocSorting[*search.Search](),
					elastic.WithTrackHits[*search.Search](false),
					elastic.WithSize[*search.Search](1),
				); {
				case err != nil:
					return models.User{}, fmt.Errorf("%w: %w", otter.ErrNotFound, err)
				case len(resp.Results) == 0:
					return models.User{}, fmt.Errorf("%w: %w", otter.ErrNotFound, elastic.ErrNotFound)
				default:
					return *resp.Results[0], nil
				}
			},
		),
	}, nil
})

// GetUser retrieves the user doc with the given id.
func (s *UserService) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUser")
	defer span.End()

	user, err := elastic.GetDoc[models.UserID, *models.User](ctx, s.store.GetIndexRO(UsersIndex), id)
	if err != nil || user == nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// GetUser retrieves the user doc with the given id.
func (s *UserService) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUser")
	defer span.End()

	users, err := elastic.SearchAll[*models.User](ctx, s.store.GetIndexRO(UsersIndex), query.MatchAll(), 5000)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user: %w", err)
	}
	return users, nil
}

// GetUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (s *UserService) GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUserByExternalID")
	defer span.End()

	ctx = context.WithValue(ctx, "users_index", s.store.GetIndexRO(UsersIndex))

	switch user, err := s.Get(ctx, externalID, s.loader); {
	case err != nil && !errors.Is(err, elastic.ErrNotFound):
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find user by external id: %w", err)
	case errors.Is(err, elastic.ErrNotFound):
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find user by external id: %w", models.ErrNotFound)
	default:
		return &user, nil
	}
}

// GetUserByEmail will retrieve a user by their email.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	switch resp, err := elastic.Search[*models.User](
		ctx,
		s.store.GetIndexRO(UsersIndex),
		elastic.WithQuery[*search.Search](
			query.Term("email", email),
		),
		elastic.WithDocSorting[*search.Search](),
		elastic.WithTrackHits[*search.Search](false),
		elastic.WithSize[*search.Search](1),
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
func (s *UserService) GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*models.User, error) {
	switch resp, err := elastic.Search[*models.User](
		ctx,
		s.store.GetIndexRO(UsersIndex),
		elastic.WithQuery[*search.Search](
			query.Terms("settings.subscription_email", emails),
		),
		elastic.WithDocSorting[*search.Search](),
		elastic.WithTrackHits[*search.Search](false),
		elastic.WithSize[*search.Search](1),
	); {
	case err != nil:
		return nil, fmt.Errorf("search by subscription email: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("search by subscription email: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// GetUserBySubscriptionID will retrieve a user from their payment subscription ID.
func (s *UserService) GetUserBySubscriptionID(ctx context.Context, id string) (*models.User, error) {
	switch resp, err := elastic.Search[*models.User](
		ctx,
		s.store.GetIndexRO(UsersIndex),
		elastic.WithQuery[*search.Search](
			query.Term("subscription.subscription_id", id),
		),
		elastic.WithDocSorting[*search.Search](),
		elastic.WithTrackHits[*search.Search](false),
		elastic.WithSize[*search.Search](1),
	); {
	case err != nil:
		return nil, fmt.Errorf("search by subscription id: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("search by subscription id: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// getUserByPurchaseToken retrieves the user associated with the given purchase token.
func (s *UserService) GetUserByPurchaseToken(ctx context.Context, token string) (*models.User, error) {
	// Retrieve the user associated with the customer ID.
	switch resp, err := elastic.Search[*models.User](
		ctx,
		s.store.GetIndexRO(UsersIndex),
		elastic.WithQuery[*search.Search](query.Term("subscription.purchase_token", token)),
		elastic.WithDocSorting[*search.Search](),
		elastic.WithTrackHits[*search.Search](false),
		elastic.WithSize[*search.Search](1),
	); {
	case err != nil:
		return nil, fmt.Errorf("search by subscription id: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("search by subscription id: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// GetUserByCustomerID retrieves the user associated with the given customer ID. It handles finding and adding customer
// details to an existing user for a new customer.
func (s *UserService) GetUserByCustomerID(ctx context.Context, id string) (*models.User, error) {
	// Retrieve the user associated with the customer ID.
	switch resp, err := elastic.Search[*models.User](
		ctx,
		s.store.GetIndexRO(UsersIndex),
		elastic.WithQuery[*search.Search](query.Term("subscription.customer_id", id)),
		elastic.WithDocSorting[*search.Search](),
		elastic.WithTrackHits[*search.Search](false),
		elastic.WithSize[*search.Search](1),
	); {
	case err != nil:
		return nil, fmt.Errorf("find user by customer id: %w", err)
	case len(resp.Results) == 0:
		return nil, fmt.Errorf("find user by customer id: %w", models.ErrNotFound)
	default:
		return resp.Results[0], nil
	}
}

// UpdateUser will apply the given updates to the user.
func (s *UserService) UpdateUser(ctx context.Context, user *models.User, updates map[string]any) error {
	ctx, span := tracer.Start(ctx, "UpdateUser")
	defer span.End()

	updates["updated_at"] = time.Now().UTC()
	if err := elastic.UpdateDoc(ctx, s.store.GetIndexRW(UsersIndex), user.GetID(), updates,
		// elastic.WithRefresh(elastic.RefreshTrue),
		elastic.WithRetryOnConflict[*update.Update](3),
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("update user: %w", err)
	}
	slogctx.FromCtx(ctx).Info("User object updated.")
	// Invalidate any cached user data.
	s.Invalidate(user.GetExternalID())
	s.Set(user.GetID(), *user)
	return nil
}

// AddUser stores and caches the given [*models.User] in the backend.
func (s UserService) AddUser(ctx context.Context, user *models.User) error {
	if err := elastic.CreateDoc(
		ctx,
		s.store.GetIndexRW(UsersIndex),
		user.GetID(),
		user,
		elastic.WithRefresh[*create.Create](refresh.Waitfor),
	); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	if _, ok := s.Set(user.GetID(), *user); !ok {
		slogctx.Warn(ctx, "Unable to cache new user.", slog.String("user_id", user.GetID()))
	}
	return nil
}

// DeleteUser deletes the user and the user's subscription objects from Elasticsearch.
func (s *UserService) DeleteUser(ctx context.Context, user *models.User) error {
	// Delete user object.
	if err := elastic.DeleteDoc(
		ctx,
		s.store.GetIndexRW(UsersIndex),
		user.GetID(),
		elastic.WithRefresh[*delete.Delete](refresh.Waitfor),
	); err != nil {
		return fmt.Errorf("delete user object: %w", err)
	}
	// Delete the user's subscriptions.
	if err := elastic.DeleteDocs(
		ctx,
		s.store.GetIndexRW(SubscriptionsIndex),
		query.Term("user_id", user.GetID()),
	); err != nil {
		return fmt.Errorf("delete user subscriptions: %w", err)
	}

	// Delete any scheduled jobs for the user.
	if err := elastic.DeleteDocs(
		ctx,
		s.store.GetIndexRW(ScheduleIndex),
		query.Term("job_data.user_id", user.GetID()),
	); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not delete scheduled jobs for user.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
	}

	s.Invalidate(user.GetID())
	return nil
}
