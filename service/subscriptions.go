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
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

var userSubscriptionsCache = otter.Must(
	&otter.Options[models.UserID, *otter.Cache[models.SubscriptionID, models.Subscription]]{
		MaximumSize: 5000,
	},
)

func loadSubscriptions(
	ctx context.Context,
	userID models.UserID,
) (*otter.Cache[models.SubscriptionID, models.Subscription], error) {
	userSubscriptionsCache, err := otter.New(&otter.Options[models.SubscriptionID, models.Subscription]{
		InitialCapacity: 3000,
		MaximumSize:     3000,
	})
	if err != nil {
		return nil, fmt.Errorf("create subscriptions cache: %w", err)
	}
	// Execute query.
	subscriptions, err := elastic.SearchAll[*models.Subscription](
		ctx,
		schema.SubscriptionsIndexRO(),
		query.Term("user_id", userID),
		3000,
	)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	if len(subscriptions) == 0 {
		return nil, otter.ErrNotFound
	}

	for subscription := range slices.Values(subscriptions) {
		userSubscriptionsCache.Set(subscription.GetID(), *subscription)
	}

	slogctx.FromCtx(ctx).Debug("Created subscriptions cache for user.")

	return userSubscriptionsCache, nil
}

// GetAllSubscriptions returns all subscriptions for the given user.
func GetAllSubscriptions(
	ctx context.Context,
	user *models.User,
) (models.Subscriptions, error) {
	subscriptionsCache, err := userSubscriptionsCache.Get(
		ctx,
		user.GetID(),
		otter.LoaderFunc[models.UserID, *otter.Cache[models.SubscriptionID, models.Subscription]](
			loadSubscriptions,
		),
	)
	switch {
	case err != nil && errors.Is(err, otter.ErrNotFound):
		return nil, fmt.Errorf("get all subscriptions: %w", models.ErrNotFound)
	case err != nil:
		return nil, fmt.Errorf("get all subscriptions: %w", err)
	}

	var subscriptions models.Subscriptions
	for subscription := range subscriptionsCache.Values() {
		subscriptions = append(subscriptions, &subscription)
	}

	if err = updateSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to update subscription dynamic info.",
			slog.Any("error", err),
		)
	}

	return subscriptions, nil
}

// GetSubscription returns the subscription that matches the given ID for the given user.
func GetSubscription(
	ctx context.Context,
	user *models.User,
	id models.SubscriptionID,
) (*models.Subscription, error) {
	subscriptions, err := GetAllSubscriptions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}
	return subscriptions.GetByID(id), nil
}

// GetSubscriptionsByID returns all subscriptions that match the given SubscriptionIDs.
func GetSubscriptionsByID(
	ctx context.Context,
	ids ...models.SubscriptionID,
) (models.Subscriptions, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	subscriptions, err := GetAllSubscriptions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	return subscriptions.FilterByIDs(ids...), nil
}

// GetSubscriptionsByFeedID returns all subscriptions that match the given FeedIDs.
func GetSubscriptionsByFeedID(
	ctx context.Context,
	ids ...models.FeedID,
) (models.Subscriptions, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	subscriptions, err := GetAllSubscriptions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	return subscriptions.FilterByFeedIDs(ids...), nil
}

// AddSubscriptions adds the given subscriptions to a user.
func AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if _, err := UpdateSubscriptions(ctx, subscriptions...); err != nil {
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
		return ElasticsearchToAPIError(err)
	}
	// Remove the subscriptions from the cache.
	if subscriptionsCache, ok := userSubscriptionsCache.GetIfPresent(user.GetID()); ok {
		for id := range slices.Values(ids) {
			subscriptionsCache.Invalidate(id)
		}
	}

	return nil
}

// UpdateSubscriptions will bulk update the given subscriptions in Elasticsearch.
func UpdateSubscriptions(
	ctx context.Context,
	subscriptions ...*models.Subscription,
) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
	resp, err := elastic.BulkUpdate(ctx, schema.SubscriptionsIndexRW(), subscriptions...)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// // Update the subscription dynamic info
	// if err = UpdateSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
	// 	slogctx.FromCtx(ctx).Warn("Could not update subscription dynamic info.",
	// 		slog.Any("errro", err),
	// 	)
	// }

	// Update the cached subscriptions.
	if subscriptionsCache, ok := userSubscriptionsCache.GetIfPresent(user.GetID()); ok {
		for subscription := range slices.Values(subscriptions) {
			subscriptionsCache.Invalidate(subscription.GetID())
			subscriptionsCache.Set(subscription.GetID(), *subscription)
		}
	}

	return resp, nil
}

// MarkSubscriptions will mark as appropriate all the given subscriptions. Marking a subscription includes updating the
// subscription data in the user object and clearing any individual item states for a subscription.
func MarkSubscriptions(
	ctx context.Context,
	mark models.Mark,
	subscriptionIDs ...models.SubscriptionID,
) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	subscriptions, err := GetAllSubscriptions(ctx,
		user,
		// WithSubscriptionIDs(subscriptionIDs...),
	)
	if err != nil {
		return fmt.Errorf("mark subscriptions: %w", err)
	}

	subscriptions = subscriptions.FilterByIDs(subscriptionIDs...)

	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			if err = MarkSubscriptions(ctx, mark, subscription.GroupData.Subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: mark group subscription: %w", err)
			}
		} else {
			subscription.Mark(user, mark)
			if _, err = UpdateSubscriptions(ctx, subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: update subscription data: %w", err)
			}
		}
	}

	return nil
}

// UpdateFavoriteSubscription changes the favorite status of a subscription by updating the user object to flag the
// subscription as appropriate.
func UpdateFavoriteSubscription(ctx context.Context, user *models.User, id models.SubscriptionID, favorite bool) error {
	subscription, err := GetSubscription(ctx, user, id)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	subscription.Favorite = favorite

	_, err = UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return ElasticsearchToAPIError(err)
	}

	return nil
}

// GetGroupSubscriptionLatestItems will return a map of latest items per subscription for the given group subscriptions.
func GetGroupSubscriptionLatestItems(
	ctx context.Context,
	user *models.User,
	count int,
	subscriptions models.Subscriptions,
) (map[models.SubscriptionID]models.Items, error) {
	groupLatestItems := make(map[models.SubscriptionID]models.Items)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for subscription := range slices.Values(subscriptions) {
		wg.Go(func() {
			// Get details of all subscriptions that comprise the group.
			childSubscriptions, err := GetAllSubscriptions(ctx, user)
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to get subscription details for group subscription.",
					slog.Any("error", err),
				)
				return
			}
			childSubscriptions = childSubscriptions.FilterByIDs(subscription.GroupData.Subscriptions...)
			// Get latest items for these subscriptions.
			latestItems, err := GetFeedLatestItems(ctx, count, childSubscriptions.GetFeedIDs()...)
			// latestItems, err := getFeedSubscriptionLatestItems(ctx, childSubscriptions, filters)
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to get latest items for group subscription.",
					slog.Any("error", err),
				)
				return
			}
			// Concat all items from all subscriptions into the group subscription items list.
			for _, items := range latestItems {
				mu.Lock()
				groupLatestItems[subscription.GetID()] = slices.Concat(groupLatestItems[subscription.GetID()], items)
				// Sort the combined items list.
				groupLatestItems[subscription.GetID()].SortByTimestamp()
				// Truncate the list to the first 3 items if greater than 3.
				if len(groupLatestItems[subscription.GetID()]) > 3 {
					groupLatestItems[subscription.GetID()] = groupLatestItems[subscription.GetID()][:3]
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return groupLatestItems, nil
}

// GetSearchSubscriptionLatestItems will return a map of latest items per subscription for the given search
// subscriptions.
func GetSearchSubscriptionLatestItems(
	ctx context.Context,
	count int,
	subscriptions models.Subscriptions,
) (map[models.SubscriptionID]models.Items, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("%w: could not find user", models.ErrCtxValueNotFound)
	}

	searchTopItems := make(map[models.SubscriptionID]models.Items)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for subscription := range slices.Values(subscriptions) {
		wg.Go(func() {
			// Generate a search query from the subscription search data.
			searchQuery, err := BuildSearchResultsQuery(
				ctx,
				user,
				&subscription.SearchData.Search,
				models.SearchResultsClause(&subscription.SearchData.Search),
			)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				slogctx.FromCtx(ctx).Warn("Could not build search query for search subscription.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
				return
			}
			// Search for items matching.
			var items models.Items
			items, _, err = SearchItems(
				ctx,
				searchQuery,
				count,
				&subscription.SearchData.Search.Sort,
				nil,
			)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				slogctx.FromCtx(ctx).Warn("Get search results for search subscription failed.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
				return
			}
			// Add to the subscription top items.
			mu.Lock()
			searchTopItems[subscription.GetID()] = items
			mu.Unlock()
		})
	}
	wg.Wait()
	return searchTopItems, nil
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
	subscriptions, err := GetAllSubscriptions(
		ctx,
		user,
		// WithEmailSubscriptionEmails(from.Address),
	)
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}

	subscriptions = subscriptions.FilterEmailIDs(from.Address)

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

// GetSubscriptionSuggestions returns subscriptions that match the given text. A set of ids can be optionally passed to
// ignore those subscriptions.
func GetSubscriptionSuggestions(
	ctx context.Context,
	text string,
	count int,
	ignoredSubscriptions []models.SubscriptionID,
) (models.Subscriptions, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Perform search.
	resp, err := elastic.Search[*models.Subscription](
		ctx,
		schema.SubscriptionsIndexRO(),
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Bool(
					query.Should(
						query.Term("type", models.SubscriptionTypeEmail),
						query.Term("type", models.SubscriptionTypeFeed),
					),
				),
			),
			query.Must(
				query.Bool(
					query.Should(
						query.SearchAsYouType(text, "customisation.nickname"),
					),
				),
			),
			query.MustNot(
				query.Terms("subscription_id", ignoredSubscriptions),
			),
		),
		elastic.WithSort(newSubscriptionSortOptions(new(models.SortMostRelevant))...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("search subscriptions: %w", models.ErrNotFound)
	}

	var subscriptions models.Subscriptions
	subscriptions = resp.Results

	err = updateSubscriptionDynamicInfo(ctx, subscriptions)
	if err != nil {
		return nil, fmt.Errorf("add dynamic info: %w", err)
	}

	return subscriptions, nil
}

func GetCategoriesForSubscriptions(
	ctx context.Context,
	subscriptionIDs ...models.SubscriptionID,
) (models.CategoryCounts, error) {
	// Retrieve user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Build query.
	var searchQuery query.Option
	if len(subscriptionIDs) == 0 {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
		)
	} else {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", subscriptionIDs),
			),
		)
	}

	// Build elastic.
	termsField := "customisation.categories.raw"
	termsCount := 200
	aggs := elastic.Aggs{
		"CategoryCounts": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}

	resp, err := elastic.Search[*models.Subscription](ctx,
		schema.SubscriptionsIndexRO(),
		searchQuery,
		elastic.WithSize(0),
		elastic.WithDocSorting(),
		elastic.WithAggregations(aggs),
	)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	categoryCounts, ok := resp.Aggregations["CategoryCounts"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"category counts aggregation invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}
	categoryCountsBuckets, ok := categoryCounts.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}

	counts := make(models.CategoryCounts, 0, len(categoryCountsBuckets))

	// Loop through the aggregation results and extract the unread count for each feed.
	for bucket := range slices.Values(categoryCountsBuckets) {
		var category models.Category
		if category, ok = bucket.Key.(string); ok {
			counts = append(counts, models.CategoryCount{Category: category, Count: int(bucket.DocCount)})
		}
	}
	return counts, nil
}

// updateSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func updateSubscriptionDynamicInfo(ctx context.Context, subscriptions models.Subscriptions) error {
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
		extraSubscriptions, err := GetSubscriptionsByID(ctx, extraIDs...)
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
			query, err := BuildSearchResultsQuery(jobCtx, user, &request, models.SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			count, err := CountItems(jobCtx, query)
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
			query, err = BuildSearchResultsQuery(jobCtx, user, &request, models.SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			if items, _, err := SearchItems(jobCtx, query, 1, &sort, nil); err == nil && len(items) > 0 {
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

// SubscriptionSorting contains the sort options for sorting subscription results.
type SubscriptionSorting struct {
	MarkedReadAt   string `json:"marked_read_at"`
	SubscriptionID string `json:"subscription_id"`
}

// SortCombinationsCaster is required to allow FeedSorting to be used as Elasticsearch sort values.
func (s *SubscriptionSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}

func newSubscriptionSortOptions(sort *models.Sort) []types.SortCombinationsVariant {
	if sort == nil {
		return []types.SortCombinationsVariant{&types.SortOptions{Doc_: types.NewScoreSort()}}
	}
	var opts []types.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "asc",
			SubscriptionID: "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "desc",
			SubscriptionID: "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&SubscriptionSorting{
				MarkedReadAt:   "asc",
				SubscriptionID: "asc",
			},
		)
	default:
		opts = append(opts, &types.SortOptions{
			Doc_: types.NewScoreSort(),
		})
	}
	return opts
}
