// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"strconv"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/mitchellh/hashstructure/v2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// GetSubscriptionOptions holds the options for fetching a user's subscriptions.
type GetSubscriptionOptions struct {
	OnlyFavorites bool
	DynamicInfo   bool
	IDs           []models.SubscriptionID
	Categories    []models.Category
	EmailIDs      []string
}

type GetSubscriptionOption func(*GetSubscriptionOptions)

func Favorites(value bool) GetSubscriptionOption {
	return func(opts *GetSubscriptionOptions) {
		opts.OnlyFavorites = value
	}
}

func WithSubscriptionIDs(ids ...models.SubscriptionID) GetSubscriptionOption {
	return func(opts *GetSubscriptionOptions) {
		if len(ids) > 0 {
			opts.IDs = ids
		}
	}
}

func WithSubscriptionCategories(categories ...models.Category) GetSubscriptionOption {
	return func(opts *GetSubscriptionOptions) {
		if len(categories) > 0 {
			opts.Categories = categories
		}
	}
}

func WithDynamicInfo(value bool) GetSubscriptionOption {
	return func(opts *GetSubscriptionOptions) {
		opts.DynamicInfo = value
	}
}

func WithEmailSubscriptionEmails(emails ...string) GetSubscriptionOption {
	return func(opts *GetSubscriptionOptions) {
		if len(emails) > 0 {
			opts.EmailIDs = emails
		}
	}
}

func GetUserSubscriptions(
	ctx context.Context,
	user *models.User,
	options ...GetSubscriptionOption,
) (models.Subscriptions, error) {
	opts := &GetSubscriptionOptions{}
	for option := range slices.Values(options) {
		option(opts)
	}

	hash, err := hashstructure.Hash(opts, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot hash options: %w", err)
	}
	cacheKey := user.GetID() + "_sub_" + strconv.FormatUint(hash, 10)

	if subscriptions, ok := subscriptionsCache.GetIfPresent(cacheKey); ok {
		slogctx.FromCtx(ctx).Debug("Cache hit!")
		return subscriptions, nil
	}

	// Build query.
	queries := []query.Option{query.Term("user_id", user.GetID())}
	if opts.OnlyFavorites {
		queries = append(queries, query.Term("favorite", true))
	}
	if len(opts.IDs) > 0 {
		queries = append(queries, query.Terms("subscription_id", opts.IDs))
	}
	if len(opts.Categories) > 0 {
		queries = append(queries, query.Terms("customisation.categories", opts.Categories))
	}
	if len(opts.EmailIDs) > 0 {
		queries = append(queries, query.Terms("email_data.email_sender_id", opts.EmailIDs))
	}

	// Execute query.
	var (
		subscriptions models.Subscriptions
	)
	subscriptions, err = elastic.SearchAll[*models.Subscription](
		ctx,
		schema.SubscriptionsIndexRO(),
		query.Bool(
			query.Filter(queries...),
		),
		models.DefaultPaginationSize,
	)
	if err != nil {
		return nil, models.ElasticsearchToAPIError(err)
	}

	// Add dynamic info if requested.
	if len(subscriptions) > 0 && opts.DynamicInfo {
		err = addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("add dynamic info: %w", err)
		}
	}

	subscriptionsCache.Set(cacheKey, subscriptions)

	return subscriptions, nil
}

// FilterUserSubscriptions returns subscriptions filtered by the given filters and paginated by the given pagination.
// Dynamic information for subscriptions will also be added.
func FilterUserSubscriptions(
	ctx context.Context,
	user *models.User,
	request *models.ListRequest,
) (models.Subscriptions, models.Pagination, error) {
	subscriptions, err := GetUserSubscriptions(
		ctx,
		user,
		WithSubscriptionIDs(request.Filters.Subscriptions...),
		WithSubscriptionCategories(request.Filters.Categories...),
		Favorites(request.Filters.OnlyFavorites),
		WithDynamicInfo(true),
	)
	if err != nil {
		return nil, "", fmt.Errorf("get all subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, "", fmt.Errorf("get all subscriptions: %w", models.ErrNotFound)
	}
	// Add dynamic info.
	err = addSubscriptionDynamicInfo(ctx, subscriptions)
	if err != nil {
		return nil, "", fmt.Errorf("add dynamic info: %w", err)
	}
	// Sort and paginate.
	var pagination models.Pagination
	if request.Pagination != nil {
		pagination = *request.Pagination
	}
	subscriptions, pagination = subscriptions.
		FilterByView(request.Filters.GetView()).
		Sort(request.Filters.Sort).
		Paginate(pagination, request.Filters.GetCount())

	return subscriptions, pagination, nil
}

// AddSubscriptions adds the given subscriptions to a user.
func AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if _, err := models.UpdateSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("update subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	if settings := user.GetSettings(); settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		if err := UpdateUser(ctx, user, map[string]any{
			"settings": settings,
		}); err != nil {
			return fmt.Errorf("update user: %w", err)
		}
	}
	return nil
}

// RemoveSubscriptions removes subscriptions with the given ID from a user.
func RemoveSubscriptions(ctx context.Context, ids ...models.SubscriptionID) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if err := elastic.DeleteDocs(ctx, schema.SubscriptionsIndexRW(),
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", ids),
			),
		),
	); err != nil {
		return models.ElasticsearchToAPIError(err)
	}
	return nil
}

// CreateSearchSubscriptions will create new SearchSubscriptions for the user from the given requests.
func CreateSearchSubscriptions(ctx context.Context, requests ...*models.SearchSubscriptionRequest) error {
	subscriptions := make(models.Subscriptions, 0, len(requests))
	for request := range slices.Values(requests) {
		slogctx.FromCtx(ctx).Debug("Creating new search subscription.",
			slog.String("feed", request.Customisation.GetNickname()),
		)
		// Generate metadata.
		subscription, err := models.NewSearchSubscription(ctx, request)
		if err != nil {
			return fmt.Errorf("create search subscription: generate subscription failed: %w", err)
		}
		err = subscription.Valid()
		if err != nil {
			return fmt.Errorf("create search subscription: invalid data: %w", err)
		}
		subscriptions = append(subscriptions, subscription)
	}
	// Add subscriptions
	if err := AddSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("create search subscription: add subscriptions failed: %w", err)
	}
	return nil
}

// GetEmailSubscription retrieves an EmailSubscription for the given user ID and email sender.
func GetEmailSubscription(ctx context.Context, user *models.User, from *mail.Address) (*models.Subscription, error) {
	// Check for an existing email subscription for the user.
	subscriptions, err := GetUserSubscriptions(ctx, user, WithEmailSubscriptionEmails(from.Address))
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}

	var subscription *models.Subscription
	switch {
	case len(subscriptions) > 1:
		// Ambiguous subscription match for sender.
		return nil, fmt.Errorf("%w: ambiguous subscription match for sender", models.ErrInvalidAPIResult)
	case len(subscriptions) == 0:
		// Create a new email subscription for this sender.
		subscription, err = models.NewEmailSubscription(ctx, user.GetID(), from)
		if err != nil {
			return nil, fmt.Errorf("create email subscription: %w", err)
		}
		if err := AddSubscriptions(ctx, subscription); err != nil {
			return nil, fmt.Errorf("add email subscription: %w", err)
		}
	default:
		// Use the existing email subscription.
		subscription = subscriptions[0]
	}

	return subscription, nil
}

// addSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func addSubscriptionDynamicInfo(ctx context.Context, subscriptions models.Subscriptions) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	var extraIDs []models.SubscriptionID
	for subscription := range slices.Values(subscriptions) {
		// Get any additional subscription info for subscriptions in group subscriptions that we didn't already fetch.
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			for id := range slices.Values(subscription.GroupData.Subscriptions) {
				if !slices.ContainsFunc(subscriptions, func(e *models.Subscription) bool {
					return e.GetID() == id
				}) {
					extraIDs = append(extraIDs, id)
				}
			}
		}
	}
	if len(extraIDs) > 0 {
		extraSubscriptions, err := models.GetSubscriptions(ctx,
			models.GetSubscriptionsByIDs(extraIDs...),
		)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			return fmt.Errorf("add subscription dynamic info: get additional subscriptions: %w", err)
		}
		subscriptions = append(subscriptions, extraSubscriptions...)
	}

	fetchJobs, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()

	// Get unread count per feed.
	var unreadCounts map[models.FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = getFeedUnreadCounts(jobCtx, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeSearch)) {
			request := subscription.SearchData.Search
			// Build query to get unread count.
			query, err := models.BuildSearchResultsQuery(jobCtx, user, &request, models.SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			count, err := models.CountItems(jobCtx, query)
			if err == nil {
				subscription.GetStats().UnreadCount = int(count)
			} else {
				slogctx.FromCtx(jobCtx).
					Warn("Add subscription dynamic info, could not get unread count for search subscription.",
						slog.String("subscription_id", subscription.GetID()),
						slog.Any("error", err),
					)
			}
			// Update query for getting last updated item (view: all, sort: newest first).
			request.View = models.ViewAll
			sort := models.SortNewestFirst
			query, err = models.BuildSearchResultsQuery(jobCtx, user, &request, models.SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			if items, _, err := models.SearchItems(jobCtx, query, 1, &sort, nil); err == nil && len(items) > 0 {
				subscription.GetStats().LastUpdate = items[0].GetTimestamp()
			} else {
				slogctx.FromCtx(jobCtx).
					Warn("Add subscription dynamic info, could not get last update for search subscription.",
						slog.String("subscription_id", subscription.GetID()),
						slog.Any("error", err),
					)
			}
		}
		return nil
	})

	// Get last update (latest item timestamp) per feed.
	var lastUpdate map[models.FeedID]time.Time
	fetchJobs.Go(func() error {
		var err error
		lastUpdate, err = getFeedLastUpdates(jobCtx, subscriptions.GetFeedIDs()...)
		if err != nil {
			return fmt.Errorf("get last update: %w", err)
		}
		return nil
	})

	var avgDailyUpdates map[models.FeedID]float64
	if user.GetSettings().ShowSubscriptionStats {
		// Get average daily updates per feed
		fetchJobs.Go(func() error {
			var err error
			avgDailyUpdates, err = getFeedAverageDailyUpdates(jobCtx, subscriptions.GetFeedIDs()...)
			if err != nil {
				return fmt.Errorf("get average daily updates: %w", err)
			}
			return nil
		})
	}

	if err := fetchJobs.Wait(); err != nil {
		return fmt.Errorf("add subscription dynamic info: run jobs: %w", err)
	}

	// For feed subscriptions, add stats.
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeFeed)) {
		subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
		subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
		if user.GetSettings().ShowSubscriptionStats {
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
		}
	}

	// For email subscriptions, add stats.
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeEmail)) {
		subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
		subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
		if user.GetSettings().ShowSubscriptionStats {
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeGroup)) {
		var avgDailyUpdates []float64
		var unreadCount int
		var lastUpdates []time.Time
		for groupSubscription := range slices.Values(subscriptions) {
			if slices.Contains(subscription.GroupData.Subscriptions, groupSubscription.GetID()) {
				if user.GetSettings().ShowSubscriptionStats {
					avgDailyUpdates = append(avgDailyUpdates, groupSubscription.GetStats().AvgDailyUpdates)
				}
				unreadCount += groupSubscription.GetStats().UnreadCount
				lastUpdates = append(lastUpdates, groupSubscription.GetStats().LastUpdate)
			}
		}
		if user.GetSettings().ShowSubscriptionStats && len(avgDailyUpdates) > 0 {
			slices.Sort(avgDailyUpdates)
			slices.Reverse(avgDailyUpdates)
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[0]
		}
		subscription.GetStats().UnreadCount = unreadCount
		// Sort by date ascending, with favorites before non-favorites.
		slices.SortFunc(lastUpdates, func(timeA, timeB time.Time) int {
			return timeA.Compare(timeB)
		})
		slices.Reverse(lastUpdates)
		subscription.GetStats().LastUpdate = lastUpdates[0]
	}

	return nil
}
