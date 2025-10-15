// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

type DataAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)
	ItemsAggregation2(ctx context.Context, query query.Option, count int, agg aggregations.Aggs) (*search.Response, error)
	UpdateUser(ctx context.Context, updates map[string]any) error
	SearchItems(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Items, Pagination, error)
}

func FilterSubscriptions(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (SubscriptionsSlice, Pagination, error) {
	// Get subscriptions by ID.
	subscriptions, err := GetSubscriptions(ctx, dataAPI, filters.GetSubscriptions()...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to filter subscriptions: %w", err)
	}
	// Filter subscriptions.
	sort := filters.GetSort()
	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(&sort)
	// Set up pagination.
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

func GetSubscriptions(ctx context.Context, dataAPI DataAPI, ids ...SubscriptionID) (SubscriptionsSlice, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}
	allFavorites := user.GetFavorites().FilterByType(FavoriteTypeSubscription)
	// Get the subscription states.
	var allMetadata SubscriptionMetadataSlice
	if len(ids) > 0 {
		allMetadata = user.GetSubscriptionMetadata().FilterByIDs(ids...)
	} else {
		allMetadata = user.GetSubscriptionMetadata()
	}
	// Get unread counts.
	unreadCounts, err := GetSubscriptionUnreadCounts(ctx, dataAPI, allMetadata)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Get feed data for subscriptions.
	feeds, err := dataAPI.GetFeeds(ctx, allMetadata.GetFeedIDs()...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Generate subscriptions from data sources.
	subscriptions := make(SubscriptionsSlice, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var metadata *SubscriptionMetadata
		var count int64
		if metadata = allMetadata.GetByFeedID(feed.GetID()); metadata == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		count = unreadCounts[metadata.GetID()]

		subscription, err := GenerateSubscription(metadata, feed, int(count), allFavorites.HasFavorite(metadata.GetID()))
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func GetSubscriptionUnreadCounts(ctx context.Context, dataAPI DataAPI, subscriptionMetadata SubscriptionMetadataSlice) (map[SubscriptionID]int64, error) {
	// Retrieve user object.
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}
	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptionMetadata))
	for m := range slices.Values(subscriptionMetadata) {
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, m))
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
	termsCount := len(subscriptionMetadata)
	aggs := aggregations.Aggs{
		"UnreadCounts": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := dataAPI.ItemsAggregation2(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	unreadCounts, ok := results.Aggregations["UnreadCounts"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get unread counts: feed aggregations invalid")
	}
	unreadCountsBuckets, ok := unreadCounts.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get unread counts: feed aggregations invalid")
	}

	stats := make(map[SubscriptionID]int64)

	for feed := range slices.Values(unreadCountsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		stats[user.GetSubscriptionMetadata().GetByFeedID(feedID).GetID()] = feed.DocCount
	}
	return stats, nil
}

func GetSubscriptionStats(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters) (map[SubscriptionID]SubscriptionStats, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("getFeedStats: %w", err)
	}

	// Get the subscription metadata.
	var metadata SubscriptionMetadataSlice
	if len(filters.Subscriptions) > 0 {
		metadata = user.GetSubscriptionMetadata().FilterByIDs(filters.Subscriptions...)
	} else {
		metadata = user.GetSubscriptionMetadata()
	}
	// Build query.
	query := query.Bool(
		query.BoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", filters.Subscriptions...),
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.GetView(), metadata...)...),
			),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(metadata)
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

	results, err := dataAPI.ItemsAggregation2(ctx, query, len(metadata), aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed stats: feed aggregations invalid")
	}
	feedStats, ok := results.Aggregations["feed"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: feed aggregations invalid")
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: feed aggregations invalid")
	}

	stats := make(map[FeedID]SubscriptionStats)

	for feed := range slices.Values(feedStatsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		updatesResult, ok := feed.Aggregations["avg_daily_updates"].(*types.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("subscription", user.GetSubscriptionMetadata().GetByFeedID(feedID).GetID()))
			continue
		}

		stats[user.GetSubscriptionMetadata().GetByFeedID(feedID).GetID()] = SubscriptionStats{
			AvgDailyUpdates: float64(*updatesResult.Value),
		}
	}
	return stats, nil
}

func MarkSubscriptions(ctx context.Context, dataAPI DataAPI, mark Mark, subscriptions ...SubscriptionID) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to mark subscriptions: %w", err)
	}
	// Mark user subscriptions.
	user.MarkSubscriptions(mark, subscriptions...)
	// Update the user.
	err = dataAPI.UpdateUser(ctx, map[string]any{
		"subscriptions": user.GetSubscriptionMetadata(),
	})
	if err != nil {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	return nil
}

func FilterArticles(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (Articles, Pagination, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to filter articles: %w", err)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	subscriptions := user.GetSubscriptionMetadata()
	if len(filters.Subscriptions) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.Subscriptions...)
	}
	query := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.GetView(), subscriptions...)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := dataAPI.SearchItems(ctx, query, filters.GetCount(), &sort, &pagination)
	if err != nil {
		return nil, "", RespErrBackend(err)
	}
	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, "", RespErrBackend(err)
	}

	return articles, pagination, nil
}

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
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}

	return articles, nil
}

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
	results, err := dataAPI.ItemsAggregation2(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := results.Aggregations["TopCategories"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid")
	}
	topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid")
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

func BuildItemsQuery(ctx context.Context, filters Filters, subscriptions ...SubscriptionID) (query.Option, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to build items query: %w", err)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	meta := user.GetSubscriptionMetadata().FilterByIDs(subscriptions...)
	return query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", meta.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.GetView(), meta...)...),
			),
		),
	), nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func buildSubscriptionQueries(user *User, view View, subscriptions ...*SubscriptionMetadata) []query.Option {
	queries := make([]query.Option, 0, len(user.Subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	switch view {
	case ViewRead:
		for _, state := range subscriptions {
			queries = append(queries, queryReadItems(user, state))
		}
	case ViewAll:
		for _, state := range subscriptions {
			queries = append(queries, queryAllItems(user, state))
		}
	case ViewUnread:
		fallthrough
	default:
		for _, state := range subscriptions {
			queries = append(queries, queryUnreadItems(user, state))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, subscription *SubscriptionMetadata) query.Option {
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Between("updated", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Terms("item_id", subscription.GetReadItems()...),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.GetUnreadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *User, subscription *SubscriptionMetadata) query.Option {
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", subscription.MarkedReadAt),
					query.Since("updated", subscription.MarkedReadAt),
					query.Terms("item_id", subscription.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.GetReadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, subscription *SubscriptionMetadata) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
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
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}
