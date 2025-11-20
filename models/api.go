// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidAPIResult = errors.New("invalid backend API result")
)

// FeedsAPI contains API methods for Feeds.
type FeedsAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)

	SearchFeeds(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Feeds, Pagination, error)
	// MultiSearchFeeds(ctx context.Context, queries ...*MultiSearchQuery) (results.MSearchResults, error)
	CreateFeed(ctx context.Context, feed *Feed) error
}

// SubscriptionsAPI contains API methods for Subscriptions.
type SubscriptionsAPI interface {
	GetAllSubscriptions(ctx context.Context, query query.Option) (Subscriptions, error)
	SearchSubscriptions(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Subscriptions, Pagination, error)
	// GetSubscriptions(ctx context.Context, ids ...SubscriptionID) (Subscriptions, error)
	// GetSubscription(ctx context.Context, id SubscriptionID) (*Subscription, error)
	UpdateSubscriptions(ctx context.Context, subscriptions ...*Subscription) (map[SubscriptionID]*bulk.OperationResponse, error)
	// UpdateSubscription(ctx context.Context, subscriptions *Subscription) error
	RemoveSubscriptions(ctx context.Context, query query.Option) error
}

// ItemsAPI contains API methods for Items.
type ItemsAPI interface {
	SearchItems(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Items, Pagination, error)
	CountItems(ctx context.Context, query query.Option) (int64, error)
	GetLastUpdatedItems(ctx context.Context, feedIDs ...FeedID) (Items, error)
	ItemsAggregation(ctx context.Context, query query.Option, count int, agg aggregations.Aggs) (*search.Response, error)
	ArchiveArticle(ctx context.Context, article *ArticleArchive) error
	UnarchiveArticle(ctx context.Context, userID UserID, itemID ItemID) error
}

// UserAPI contains API methods for Users.
type UserAPI interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, id UserID, updates map[string]any) error
}

// DataAPI contains all methods for data API access.
type DataAPI interface {
	FeedsAPI
	ItemsAPI
	UserAPI
	SubscriptionsAPI
	GetAPI() *elasticsearch.TypedClient
}

// UpdateFavoriteSubscription changes the favorite status of a subscription by updating the user object to flag the
// subscription as appropriate.
func UpdateFavoriteSubscription(ctx context.Context, dataAPI DataAPI, id SubscriptionID, favorite bool) error {
	subscription, err := GetSubscription(ctx, dataAPI, id)
	if err != nil {
		return fmt.Errorf("update favorite subscription: get subscription: %w", err)
	}

	subscription.Favorite = favorite

	_, err = dataAPI.UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return fmt.Errorf("update favorite subscription: update subscription: %w", err)
	}

	return nil
}

// UpdateFavoriteArticle changes the favorite status of an article. For adding a favorite article, the content is stored
// in a separate and the user object is updated with a link to the content. For removing a favorite, the stored content
// is removed and user object updated appropriately.
func UpdateFavoriteArticle(ctx context.Context, dataAPI DataAPI, user *User, id ItemID, favorite bool) error {
	switch favorite {
	case true:
		// Don't do anything if article is already a favorite.
		if slices.Contains(user.ItemFavorites, id) {
			return ErrUserAlreadyFavorited
		}
		// Get the article details.
		articles, err := GetArticles(ctx, dataAPI, id)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		if len(articles) != 1 {
			return ErrInvalidAPIResult
		}
		article := articles[0]
		// Archive the article.
		archive, err := NewArchivedArticle(user.GetID(), article.GetSubscriptionID(), &article.Item)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		err = dataAPI.ArchiveArticle(ctx, archive)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		// Update the list of favorites items in the user object
		user.ItemFavorites = append(user.ItemFavorites, id)
		err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": user.ItemFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
	case false:
		err := dataAPI.UnarchiveArticle(ctx, user.GetID(), id)
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
		newFavorites := slices.DeleteFunc(user.ItemFavorites, func(e ItemID) bool {
			return e == id
		})
		err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": newFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
	}
	return nil
}

// GetSubscription returns the subscription that matches the given ID. Note: no dynamic info is generated for the
// subscription (use AddSubscriptionDynamicInfo after calling this method if needed).
func GetSubscription(ctx context.Context, dataAPI DataAPI, id SubscriptionID) (*Subscription, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get subscription: get user data: %w", err)
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Term("subscription_id", id),
		),
	)
	subscriptions, _, err := dataAPI.SearchSubscriptions(ctx, subscriptionQuery, 1, nil, nil)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get subscription: %w", err)
	case len(subscriptions) == 0:
		return nil, ErrNotFound
	case len(subscriptions) != 1:
		return nil, fmt.Errorf("get subscription: %w: too many subscriptions", ErrInvalidAPIResult)
	}

	return subscriptions[0], nil
}

// GetSubscriptions returns the subscriptions that match the given IDs. Note: no dynamic info is generated for the
// subscriptions (use AddSubscriptionDynamicInfo after calling this method if needed).
func GetSubscriptions(ctx context.Context, dataAPI DataAPI, ids ...SubscriptionID) (Subscriptions, error) {
	// Get subscriptions by ID.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: get user data: %w", err)
	}

	// Suggestions query will match in title/description/categories across all feed subscriptions.
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", ids...),
		),
	)

	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, ErrNotFound
	}
	return subscriptions, nil
}

// FilterSubscriptions returns subscriptions filtered by the given filters and paginated by the given pagination.
// Dynamic information for subscriptions will also be added.
func FilterSubscriptions(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (Subscriptions, Pagination, error) {
	// Get subscriptions by ID.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: get user data: %w", err)
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", filters.Subscriptions...),
			// query.Term("favorite", filters.OnlyFavorites),
			query.Terms("categories", filters.GetCategories()...),
		),
	)
	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionQuery)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, "", ErrNotFound
	}
	// Add dynamic info.
	err = AddSubscriptionDynamicInfo(ctx, dataAPI, subscriptions)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: could not add dynamic info: %w", err)
	}
	// Sort and paginate.
	subscriptions, pagination = subscriptions.
		FilterByView(filters.GetView()).
		FilterByFavorites(filters.OnlyFavorites).
		Sort(filters.Sort).
		Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

// GetSubscriptionSuggestions returns subscriptions that match the given text. Note: no dynamic info is generated for the
// subscriptions (use AddSubscriptionDynamicInfo after calling this method if needed).
func GetSubscriptionSuggestions(ctx context.Context, dataAPI DataAPI, text string) (Subscriptions, error) {
	// Get subscriptions by ID.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: get user data: %w", err)
	}

	// Suggestions query will match in title/description/categories across all feed subscriptions.
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
		),
		query.Must(
			query.Bool(
				query.Should(
					query.SearchAsYouType(text, "customisation.nickname"),
				),
			),
		),
	)

	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, ErrNotFound
	}
	return subscriptions, nil
}

// AddSubscriptions adds the given subscriptions to a user.
func AddSubscriptions(ctx context.Context, dataAPI DataAPI, subscriptions ...*Subscription) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("add subscriptions: get user data: %w", err)
	}
	_, err = dataAPI.UpdateSubscriptions(ctx, subscriptions...)
	if err != nil {
		return fmt.Errorf("add subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	settings := user.GetSettings()
	if settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
			"settings": settings,
		})
		if err != nil {
			return fmt.Errorf("add subscriptions: update user: %w", err)
		}
	}
	return nil
}

// RemoveSubscriptions removes subscriptions with the given ID from a user.
func RemoveSubscriptions(ctx context.Context, dataAPI DataAPI, ids ...SubscriptionID) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("remove subscriptions: get user data: %w", err)
	}
	query := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", ids...),
		),
	)
	err = dataAPI.RemoveSubscriptions(ctx, query)
	if err != nil {
		return fmt.Errorf("remove subscriptions: %w", err)
	}
	return nil
}

// CreateFeedSubscriptions will create new FeedSubscriptions for the user from the given requests.
func CreateFeedSubscriptions(ctx context.Context, dataAPI DataAPI, results ...*AddFeedSubscriptionResult) error {
	if len(results) == 0 {
		return nil
	}
	subscriptions := make(Subscriptions, 0, len(results))
	for result := range slices.Values(results) {
		slogctx.FromCtx(ctx).Debug("Creating new subscription.",
			slog.String("feed", result.Feed.GetTitle()),
			slog.String("url", result.Feed.GetLink()),
		)
		// Generate metadata.
		subscription, err := NewFeedSubscription(ctx, &result.Feed, &result.Request)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not create subscription",
				slog.Any("error", err))
			result.Error = fmt.Errorf("unable to create subscription: invalid metadata: %w", err)
			continue
		}
		err = subscription.Valid()
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not create subscription",
				slog.Any("error", err))
			result.Error = fmt.Errorf("unable to create subscription: invalid metadata: %w", err)
			continue
		}
		result.Subscription = *subscription
		subscriptions = append(subscriptions, subscription)
		result.Message = *NewSuccessMessage("Subscription Created: "+result.Feed.GetTitle(), "Articles will be fetched shortly...")
	}
	// Add subscriptions
	err := AddSubscriptions(ctx, dataAPI, subscriptions...)
	if err != nil {
		return fmt.Errorf("unable to create subscriptions: %w", err)
	}
	return nil
}

// CreateSearchSubscriptions will create new SearchSubscriptions for the user from the given requests.
func CreateSearchSubscriptions(ctx context.Context, dataAPI DataAPI, requests ...*SearchSubscriptionRequest) error {
	subscriptions := make(Subscriptions, 0, len(requests))
	for request := range slices.Values(requests) {
		slogctx.FromCtx(ctx).Debug("Creating new search subscription.",
			slog.String("feed", request.Customisation.Nickname),
		)
		// Generate metadata.
		subscription, err := NewSearchSubscription(ctx, request)
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
	err := AddSubscriptions(ctx, dataAPI, subscriptions...)
	if err != nil {
		return fmt.Errorf("create search subscription: add subscriptions failed: %w", err)
	}
	return nil
}

// AddSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func AddSubscriptionDynamicInfo(ctx context.Context, dataAPI DataAPI, subscriptions Subscriptions) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("add subscription dynamic info: get user data: %w", err)
	}

	// Get any additional subscription info for subscriptions in group subscriptions that we didn't already fetch.
	var extraIDs []SubscriptionID
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == SubscriptionTypeGroup {
			for id := range slices.Values(subscription.GroupData.Subscriptions) {
				if !slices.ContainsFunc(subscriptions, func(e *Subscription) bool {
					return e.GetID() == id
				}) {
					extraIDs = append(extraIDs, id)
				}
			}
		}
	}
	extraSubscriptions, err := dataAPI.GetAllSubscriptions(ctx, query.Terms("subscription_id", extraIDs...))
	if err != nil {
		return fmt.Errorf("add subscription dynamic info: get additional subscriptions: %w", err)
	}
	subscriptions = append(subscriptions, extraSubscriptions...)

	fetchJobs, ctx := errgroup.WithContext(ctx)

	// Get unread count per feed.
	var unreadCounts map[FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = getFeedUnreadCounts(ctx, dataAPI, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		user, err := UserFromCtx(ctx)
		if err != nil {
			return fmt.Errorf("add subscription dynamic info: get search subscription info: get user data: %w", err)
		}
		for subscription := range slices.Values(subscriptions) {
			if subscription.GetSubscriptionType() != SubscriptionTypeSearch {
				continue
			}
			search := subscription.SearchData.Search
			// Build query to get unread count.
			query, err := BuildSearchResultsQuery(ctx, dataAPI, user, &search)
			if err != nil {
				return fmt.Errorf("add subscription dynamic info: build search subscription %s query: %w", subscription.GetID(), err)
			}
			count, err := dataAPI.CountItems(ctx, query)
			if err == nil {
				subscription.Stats.UnreadCount = int(count)
			} else {
				slogctx.FromCtx(ctx).Warn("Add subscription dynamic info, could not get unread count for search subscription.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
			}
			// Update query for getting last updated item (view: all, sort: newest first).
			search.View = ViewAll
			sort := SortNewestFirst
			query, err = BuildSearchResultsQuery(ctx, dataAPI, user, &search)
			if err != nil {
				return fmt.Errorf("add subscription dynamic info: build search subscription %s query: %w", subscription.GetID(), err)
			}
			items, _, err := dataAPI.SearchItems(ctx, query, 1, &sort, nil)
			if err == nil {
				if len(items) > 0 {
					subscription.Stats.LastUpdate = items[0].GetTimestamp()
				}
			} else {
				slogctx.FromCtx(ctx).Warn("Add subscription dynamic info, could not get last update for search subscription.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
			}
		}
		return nil
	})

	// Get last update (latest item timestamp) per feed.
	var lastUpdate map[FeedID]time.Time
	fetchJobs.Go(func() error {
		var err error
		lastUpdate, err = getFeedLastUpdates(ctx, dataAPI, subscriptions.GetFeedIDs()...)
		if err != nil {
			return fmt.Errorf("get last update: %w", err)
		}
		return nil
	})

	var avgDailyUpdates map[FeedID]float64
	if user.GetSettings().ShowSubscriptionStats {
		// Get average daily updates per feed
		fetchJobs.Go(func() error {
			var err error
			avgDailyUpdates, err = getFeedAverageDailyUpdates(ctx, dataAPI, subscriptions.GetFeedIDs()...)
			if err != nil {
				return fmt.Errorf("get average daily updates: %w", err)
			}
			return nil
		})
	}

	err = fetchJobs.Wait()
	if err != nil {
		return fmt.Errorf("add subscription dynamic info: run jobs: %w", err)
	}

	// For feed subscriptions, add stats.
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == SubscriptionTypeFeed {
			// Add stats for feed subscriptions.
			subscription.Stats.UnreadCount = int(unreadCounts[subscription.FeedData.FeedID])
			subscription.Stats.LastUpdate = lastUpdate[subscription.FeedData.FeedID]
			if user.GetSettings().ShowSubscriptionStats {
				subscription.Stats.AvgDailyUpdates = avgDailyUpdates[subscription.FeedData.FeedID]
			}
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == SubscriptionTypeGroup {
			var avgDailyUpdates []float64
			var unreadCount int
			var lastUpdates []time.Time
			for groupSubscription := range slices.Values(subscriptions) {
				if slices.Contains(subscription.GroupData.Subscriptions, groupSubscription.GetID()) {
					if user.GetSettings().ShowSubscriptionStats {
						avgDailyUpdates = append(avgDailyUpdates, groupSubscription.Stats.AvgDailyUpdates)
					}
					unreadCount += groupSubscription.Stats.UnreadCount
					lastUpdates = append(lastUpdates, groupSubscription.Stats.LastUpdate)
				}
			}
			if user.GetSettings().ShowSubscriptionStats {
				slices.Sort(avgDailyUpdates)
				slices.Reverse(avgDailyUpdates)
				subscription.Stats.AvgDailyUpdates = avgDailyUpdates[0]
			}
			subscription.Stats.UnreadCount = unreadCount
			// Sort by date ascending, with favorites before non-favorites.
			slices.SortFunc(lastUpdates, func(timeA, timeB time.Time) int {
				return timeA.Compare(timeB)
			})
			slices.Reverse(lastUpdates)
			subscription.Stats.LastUpdate = lastUpdates[0]
		}
	}

	return nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func getFeedAverageDailyUpdates(ctx context.Context, dataAPI DataAPI, ids ...FeedID) (map[FeedID]float64, error) {
	// Build query.
	query := query.Bool(
		query.BoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", ids...),
			// Must be published within last month.
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(ids)
	dateHistoField := "@timestamp"
	dateFormat := "yyyy-MM-dd"
	aggs := aggregations.Aggs{
		"feed": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
			Aggregations: map[string]types.Aggregations{
				"updates_per_day": {
					DateHistogram: &types.DateHistogramAggregation{
						Field:            &dateHistoField,
						CalendarInterval: &calendarinterval.Day,
						Format:           &dateFormat,
					},
				},
				"avg_daily_updates": {
					AvgBucket: &types.AverageBucketAggregation{
						BucketsPath: "updates_per_day._count",
					},
				},
			},
		},
	}

	results, err := dataAPI.ItemsAggregation(ctx, query, len(ids), aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}
	feedStats, ok := results.Aggregations["feed"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", ErrInvalidAPIResult)
	}

	stats := make(map[FeedID]float64)

	for feed := range slices.Values(feedStatsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		updatesResult, ok := feed.Aggregations["avg_daily_updates"].(*types.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("feed_id", feedID))
			continue
		}

		stats[feedID] = float64(*updatesResult.Value)
	}

	return stats, nil
}

func getFeedUnreadCounts(ctx context.Context, dataAPI DataAPI, subscriptions Subscriptions) (map[FeedID]int64, error) {
	// Retrieve user object.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}
	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
			continue
		}
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, subscription))
	}
	// Build query.
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(subscriptions)
	aggs := aggregations.Aggs{
		"UnreadCounts": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	unreadCounts, ok := results.Aggregations["UnreadCounts"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: UnreadCounts aggregations invalid: %w", ErrInvalidAPIResult)
	}
	unreadCountsBuckets, ok := unreadCounts.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: UnreadCounts aggregations invalid: %w", ErrInvalidAPIResult)
	}

	stats := make(map[SubscriptionID]int64)

	for feed := range slices.Values(unreadCountsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		stats[feedID] = feed.DocCount
	}
	return stats, nil
}

func getFeedLastUpdates(ctx context.Context, dataAPI DataAPI, ids ...FeedID) (map[FeedID]time.Time, error) {
	items, err := dataAPI.GetLastUpdatedItems(ctx, ids...)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed last updates: %w", err)
	}

	updates := make(map[FeedID]time.Time)

	for item := range slices.Values(items) {
		updates[item.GetFeedID()] = item.GetTimestamp()
	}

	return updates, nil
}

func ProcessSubscriptionRequest(ctx context.Context, dataAPI DataAPI, request *AddFeedSubscriptionRequest, resultsCh chan AddFeedSubscriptionResult) {
	result := AddFeedSubscriptionResult{
		Request: *request,
	}
	// Try to match request URL to an existing feed
	var feed *Feed
	feeds, _, err := dataAPI.SearchFeeds(ctx, query.Term("source_urls", request.GetURL()), 1, nil, nil)
	if err != nil {
		result.Error = err
		result.Message = *NewErrorMessage("Unable to determine existing subscription status", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if len(feeds) == 1 {
		feed = feeds[0]
	}

	// If no existing feed, create a new one.
	if feed == nil {
		slogctx.FromCtx(ctx).Debug("Parsing url", slog.String("url", request.GetURL()))
		newFeed, err := NewFeedFromURL(ctx, request.GetURL())
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = validation.Validate.Struct(newFeed)
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = CreateFeed(ctx, dataAPI, newFeed)
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create new feed for subscription", "The backend produced an error. This might be temporary, please try again.")
			resultsCh <- result
			return
		}
		slogctx.FromCtx(ctx).Debug("Created new feed for request.",
			slog.String("name", newFeed.GetTitle()),
			slog.String("urls", strings.Join(newFeed.GetSourceURLs(), ",")),
		)
		feed = newFeed
	}

	user, err := UserFromCtx(ctx)
	if err != nil {
		result.Error = err
		result.Message = *NewErrorMessage("Unable to check for existing subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Term("type", SubscriptionTypeFeed),
			query.Term("feed_data.feed_id", feed.GetID()),
		),
	)
	subscriptions, _, err := dataAPI.SearchSubscriptions(ctx, subscriptionQuery, 1, nil, nil)
	if err != nil {
		result.Error = err
		result.Message = *NewErrorMessage("Unable to check for existing subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if len(subscriptions) > 0 {
		result.Error = fmt.Errorf("already subscribed")
		result.Message = *NewWarningMessage("Already subscribed to feed", feed.GetTitle()+" ("+request.URL+")")
		resultsCh <- result
		return
	}

	// Add the feed details to the result.
	result.Feed = *feed
	// Send the result back through the channel.
	resultsCh <- result
}

// CreateFeed stores a new Feed.
func CreateFeed(ctx context.Context, dataAPI DataAPI, feed *Feed) error {
	err := dataAPI.CreateFeed(ctx, feed)
	if err != nil {
		return fmt.Errorf("unable to create feed: %w", err)
	}
	return nil
}

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func FilterArticles(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (Articles, Pagination, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("filter articles: get user data: %w", err)
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", filters.GetSubscriptions()...),
		),
	)
	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionQuery)
	if err != nil {
		return nil, "", fmt.Errorf("filter articles: get subscriptions: %w", err)
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, "", ErrNotFound
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	articleQuery := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := dataAPI.SearchItems(ctx, articleQuery, filters.GetCount(), &sort, &pagination)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}
	// Generate articles.
	articles, err := GenerateArticles(ctx, dataAPI, items)
	if err != nil {
		return nil, "", fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, pagination, nil
}

// GetArticles generates Article objects from the Items with the given IDs.
func GetArticles(ctx context.Context, dataAPI DataAPI, itemIDs ...ItemID) (Articles, error) {
	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)
	items, _, err := dataAPI.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}
	articles, err := GenerateArticles(ctx, dataAPI, items)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}

	return articles, nil
}

// GetArticleTopCategories performs an aggregation to return the top Item categories across the given Feeds.
func GetArticleTopCategories(ctx context.Context, dataAPI DataAPI, feeds ...FeedID) ([]Category, error) {
	// Build query.
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feeds...),
		),
	)
	// Build aggregations.
	termsField := "categories.raw"
	termsCount := 10
	aggs := aggregations.Aggs{
		"TopCategories": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := results.Aggregations["TopCategories"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}
	topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}

	topCategories := make([]Category, 0)

	for bucket := range slices.Values(topCategoriesBuckets) {
		category, ok := bucket.Key.(Category)
		if ok {
			topCategories = append(topCategories, category)
		}
	}

	return topCategories, nil
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func FindSimilarArticles(ctx context.Context, dataAPI DataAPI, itemIDs ...ItemID) (Articles, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("find similar articles: get user data: %w", err)
	}
	subscriptionsQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
		),
	)
	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionsQuery)
	switch {
	case err != nil:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", ErrNotFound)
	}
	// Build the More Like This query.
	// TODO: tweak values and fields for optimum results matching...
	var (
		minTermFreq   = 1
		maxQueryTerms = 12
	)
	mlt := query.NewMoreLikeThisQuery("similar_articles")
	mlt.LikeDocs(itemIDs...)
	mlt.Fields = []string{"title", "categories.raw", "author"}
	mlt.MinTermFreq = &minTermFreq
	mlt.MaxQueryTerms = &maxQueryTerms
	// Build query
	similarQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, ViewUnread, subscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := SortMostRelevant
	items, _, err := dataAPI.SearchItems(ctx, similarQuery, 15, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := GenerateArticles(ctx, dataAPI, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// BuildItemsQuery generates a query to fetch the Items that match the given Filters from the given Subscriptions.
func BuildItemsQuery(ctx context.Context, dataAPI DataAPI, filters Filters, subscriptionIDs ...SubscriptionID) (query.Option, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to build items query: %w", err)
	}

	subscriptionsQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", subscriptionIDs...),
		),
	)
	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionsQuery)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", ErrNotFound)
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	return query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	), nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildSubscriptionQueries(user *User, view View, subscriptions Subscriptions) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
			continue
		}
		switch view {
		case ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func BuildSearchResultsQuery(ctx context.Context, dataAPI DataAPI, user *User, request *SearchRequest) (query.Option, error) {
	// var err error
	var loc *time.Location
	var err error
	if request.Timezone != "" {
		loc, err = time.LoadLocation(request.Timezone)
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	} else {
		loc, err = time.LoadLocation("UTC")
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	}
	var since time.Time
	var pivot string
	switch request.PublishedWithin {
	case SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
		pivot = "30m"
	case SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
		pivot = "6h"
	case SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
		pivot = "12h"
	case SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	case SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
		pivot = "14d"
	default: // default to one week.
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	}

	subscriptionsQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", request.Subscriptions...),
		),
	)
	subscriptions, err := dataAPI.GetAllSubscriptions(ctx, subscriptionsQuery)
	switch {
	case err != nil:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", ErrNotFound)
	}

	return query.Bool(
		query.Filter(
			// Must be in the given user subscriptions.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, request.View, subscriptions)...),
			),
			// Must be published/updated since the given time.
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		// Boost documents closer to the current time.
		query.Should(
			query.Distance("published", pivot, "now"),
			query.Distance("updated", pivot, "now"),
		),
		// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
		// search suggestion).
		query.Must(
			// Search across title, description and content fields, with preference for match in that order (via field
			// boosting).
			query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// Search in categories.
			query.SimpleQueryString(request.Categories, "", "categories"),
			// Search in authors, contributors.
			query.SimpleQueryString(request.Authors, "", "authors", "contributors"),
		),
	), nil
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, subscription *Subscription) query.Option {
	if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
		return nil
	}
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.FeedID+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.FeedID),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Between("updated", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Terms("item_id", subscription.FeedData.GetReadItems()...),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.FeedData.GetUnreadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *User, subscription *Subscription) query.Option {
	if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
		return nil
	}
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", subscription.MarkedReadAt),
					query.Since("updated", subscription.MarkedReadAt),
					query.Terms("item_id", subscription.FeedData.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.FeedData.GetReadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, subscription *Subscription) query.Option {
	if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
		return nil
	}
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// type MultiSearchQuery struct {
// 	Name       string
// 	Index      string
// 	Query      query.Option
// 	Sort       *Sort
// 	Pagination *Pagination
// 	Size       int
// }
