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
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel/codes"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

// UserService holds a cache and backend connection for handling [models.User] objects.
type UserService struct {
	*otter.Cache[string, models.User]

	osc *opensearch.Client

	loader otter.LoaderFunc[string, models.User]

	closer closeOnce
}

// LoadUsersService loads a service that can manipulate user objects in the backend store.
func LoadUsersService(ctx context.Context, osc *opensearch.Client) (*UserService, error) {
	if osc == nil {
		return nil, errors.New("no opensearch client")
	}
	return &UserService{
		Cache: otter.Must(&otter.Options[string, models.User]{
			MaximumSize: 100,
			ExpiryCalculator: otter.ExpiryAccessing[string, models.User](
				60 * time.Second,
			),
		}),
		osc: osc,
		loader: otter.LoaderFunc[string, models.User](
			func(ctx context.Context, id string) (models.User, error) {
				resp, err := search.Search(ctx, osc, osc.IndexReadAlias(schema.UsersIndex),
					search.WithQuery(
						search.TermQuery("external_user_id", id),
					),
					search.WithSize(1),
				)
				switch {
				case err == nil:
					if users, _, err := resp.ExtractHits[*models.User](); err != nil {
						return models.User{}, fmt.Errorf("extract user: %w", err)
					} else {
						return *users[0], nil
					}
				case isNotFound(err):
					return models.User{}, fmt.Errorf("get doc: %w: %w", otter.ErrNotFound, err)
				default:
					return models.User{}, fmt.Errorf("get doc: %w", err)
				}
			},
		),
	}, nil
}

func (s *UserService) Close() error {
	// The shutdown context must not derive from the (already cancelled) signal context.
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		20*time.Second,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, slog.Default())
	defer cancel()

	slogctx.Info(shutdownCtx, "Shutting down users service")

	return s.closer.do(func() error {
		s.Cache.CleanUp()
		return nil
	})
}

// GetUser retrieves the user doc with the given id.
func (s *UserService) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUser")
	defer span.End()

	user, err := doc.Get[models.UserID, *models.User](ctx, s.osc.Doc, s.osc.IndexReadAlias(schema.UsersIndex), id)
	switch {
	case err != nil:
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user: %w", err)
	case user == nil:
		return nil, models.ErrNotFound
	}
	return user, nil
}

// GetAllUsers retrieves a slice of [models.User] for all users in the backend.
func (s *UserService) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUser")
	defer span.End()

	resp, err := search.SearchAll(
		ctx,
		s.osc,
		s.osc.IndexReadAlias(schema.UsersIndex),
		2500,
		[]search.QueryOption{search.MatchAllQuery()},
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user: %w", err)
	}
	users, err := resp.ExtractHits[*models.User]()
	if err != nil {
		return nil, fmt.Errorf("extract users: %w")
	}

	return users, nil
}

// GetUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (s *UserService) GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "GetUserByExternalID")
	defer span.End()

	switch user, err := s.Get(ctx, externalID, s.loader); {
	case err != nil && !errors.Is(err, otter.ErrNotFound):
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find user by external id: %w", err)
	case errors.Is(err, otter.ErrNotFound):
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("find user by external id: %w", models.ErrNotFound)
	default:
		return &user, nil
	}
}

// GetUserByEmail will retrieve a user by their email.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.getUserBy(ctx, search.TermQuery("email", email))
}

// GetUserBySubscriptionEmail will retrieve a user from their Foragd newsletter subscription email.
func (s *UserService) GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*models.User, error) {
	return s.getUserBy(ctx, search.TermsQuery("settings.subscription_email", emails))
}

// GetUserBySubscriptionID will retrieve a user from their payment subscription ID.
func (s *UserService) GetUserBySubscriptionID(ctx context.Context, id string) (*models.User, error) {
	return s.getUserBy(ctx, search.TermQuery("subscription.subscription_id", id))
}

// GetUserByPurchaseToken retrieves the [models.User] associated with the given purchase token.
func (s *UserService) GetUserByPurchaseToken(ctx context.Context, token string) (*models.User, error) {
	return s.getUserBy(ctx, search.TermQuery("subscription.purchase_token", token))
}

// GetUserByCustomerID retrieves the user associated with the given customer ID. It handles finding and adding customer
// details to an existing user for a new customer.
func (s *UserService) GetUserByCustomerID(ctx context.Context, id string) (*models.User, error) {
	return s.getUserBy(ctx, search.TermQuery("subscription.customer_id", id))
}

func (s *UserService) getUserBy(ctx context.Context, query search.QueryOption) (*models.User, error) {
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.UsersIndex),
		search.WithQuery(
			query,
		),
		search.WithSize(1),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSort(search.NewSortOptions(search.WithSortField("_doc"))),
	)
	switch {
	case err == nil && len(resp.Hits.Hits) > 0:
		if users, _, err := resp.ExtractHits[*models.User](); err != nil {
			return nil, fmt.Errorf("extract user: %w", err)
		} else {
			return users[0], nil
		}
	case err == nil && len(resp.Hits.Hits) == 0:
		return nil, fmt.Errorf("get doc: %w: %w", models.ErrNotFound, err)
	default:
		return nil, fmt.Errorf("get doc: %w", err)
	}
}

// UpdateUser will apply the given updates to the user.
func (s *UserService) UpdateUser(ctx context.Context, user *models.User, updates map[string]any) error {
	ctx, span := tracer.Start(ctx, "UpdateUser")
	defer span.End()

	updates["updated_at"] = time.Now().UTC()
	if err := doc.Update(
		ctx,
		s.osc.Doc,
		s.osc.IndexWriteAlias(schema.UsersIndex),
		user.GetID(),
		updates,
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
func (s *UserService) AddUser(ctx context.Context, user *models.User) error {
	if err := doc.Create(
		ctx,
		s.osc.Doc,
		s.osc.IndexWriteAlias(schema.UsersIndex),
		user,
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
	if err := doc.Delete(
		ctx,
		s.osc.Doc,
		s.osc.IndexWriteAlias(schema.UsersIndex),
		user.GetID(),
		doc.WithDeleteRefresh(opensearchapi.RefreshWaitFor),
	); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	// Delete the user's subscriptions.
	if err := doc.DeleteByQuery(
		ctx,
		s.osc,
		s.osc.IndexWriteAlias(schema.SubscriptionsIndex),
		doc.WithDeleteByQuery(search.TermQuery("user_id", user.GetID())),
	); err != nil {
		return fmt.Errorf("delete user subscriptions: %w", err)
	}

	// Delete any scheduled jobs for the user.
	if err := doc.DeleteByQuery(
		ctx,
		s.osc,
		s.osc.IndexWriteAlias(schema.SchedulerIndex),
		doc.WithDeleteByQuery(search.TermQuery("job_data.user_id", user.GetID())),
	); err != nil {
		return fmt.Errorf("delete scheduler user jobs: %w", err)
	}

	s.Invalidate(user.GetID())
	return nil
}
