// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

var ErrNotFound = errors.New("not found")

// FeedsAPI contains API methods for Feeds.
type FeedsAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)
	SearchFeeds(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Feeds, Pagination, error)
	CreateFeed(ctx context.Context, feed *Feed) error
}

// ItemsAPI contains API methods for Items.
type ItemsAPI interface {
	SearchItems(ctx context.Context, query query.Option, count int, sort *Sort, pagination *Pagination) (Items, Pagination, error)
	ItemsAggregation(ctx context.Context, query query.Option, count int, agg aggregations.Aggs) (*search.Response, error)
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
}

// TODO: set account level appropriately.
func CreateUser(ctx context.Context, dataAPI DataAPI, externalID, email string) error {
	user := NewUser(externalID, email, "auth0", UserLevelStandard)
	valid, err := user.Valid(ctx)
	if err != nil || !valid {
		return fmt.Errorf("cannot create local user: %w", err)
	}
	err = dataAPI.CreateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("unable to create user: %w", err)
	}
	return nil
}

func UpdateUser(ctx context.Context, dataAPI DataAPI, request *EditUserRequest) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve current user: %w", err)
	}
	// For the following fields, assume that if the backend value is different from the local value, it was updated on
	// the backend. In such cases, replace the local value.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if user.AvatarURL != request.AvatarURL {
		updates["avatar_url"] = request.AvatarURL
	}
	// Overwrite local nickname with remote nickname if different
	if user.Nickname != request.Nickname {
		updates["nickname"] = request.Nickname
	}
	// Overwrite local email with remote email if different
	if user.Email != request.Email {
		updates["email"] = request.Email
	}
	if len(updates) > 0 {
		// Update the user object.
		err := dataAPI.UpdateUser(ctx, user.GetID(), updates)
		if err != nil {
			return fmt.Errorf("could not update user object: %w", err)
		}
	}
	return nil
}

func FilterSubscriptions(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (Subscriptions, Pagination, error) {
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

func CreateSubscriptions(ctx context.Context, dataAPI DataAPI, results ...*SubscriptionResult) error {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to create subscription: could not determine user: %w", err)
	}
	subscriptions := make(Subscriptions, 0, len(results))
	for result := range slices.Values(results) {
		// Generate metadata.
		subscription := NewSubscription(&result.Feed, &result.Request, false)
		subscription.Mark(MarkRead, user.GetMaxHistory())
		valid, err := subscription.Valid()
		if err != nil || !valid {
			result.Error = fmt.Errorf("unable to create subscription: invalid metadata: %w", err)
			continue
		}
		result.Subscription = *subscription
		subscriptions = append(subscriptions, subscription)
		result.Message = *NewSuccessMessage("Subscription Created: "+result.Feed.GetTitle(), "Articles will be fetched shortly...")
	}
	// Add metadata to user.
	user.AddSubscriptions(subscriptions...)
	// Disable onboarding once a subscription has been added.
	settings := user.GetSettings()
	if settings.ShowOnboarding {
		settings.ShowOnboarding = false
	}
	// Update the user object.
	err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"settings":      settings,
	})
	if err != nil {
		return fmt.Errorf("unable to create subscriptions: %w", err)
	}
	return nil
}

func GetSubscriptions(ctx context.Context, dataAPI DataAPI, ids ...SubscriptionID) (Subscriptions, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not determine user: %w", err)
	}
	// Get the subscription states.
	var subscriptions Subscriptions
	if len(ids) > 0 {
		subscriptions = user.GetSubscriptions().FilterByIDs(ids...)
	} else {
		subscriptions = user.GetSubscriptions()
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, nil
	}
	// // Get unread counts.
	// unreadCounts, err := getSubscriptionUnreadCounts(ctx, dataAPI, allMetadata)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not retrieve unread counts: %w", err)
	// }
	// Get subscription stats.
	stats, err := GetSubscriptionStats(ctx, dataAPI, subscriptions)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve stats: %w", err)
	}
	// Get feed data for subscriptions.
	feeds, err := dataAPI.GetFeeds(ctx, subscriptions.GetFeedIDs()...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Add feed and stats data to subscriptions.
	for feed := range slices.Values(feeds) {
		if subscription := subscriptions.GetByFeedID(feed.GetID()); subscription != nil {
			subscription.Feed = *feed
			subscription.Stats = stats[subscription.GetID()]
		}
	}

	return subscriptions, nil
}

func GetSubscriptionStats(ctx context.Context, dataAPI DataAPI, subscriptions Subscriptions) (map[SubscriptionID]SubscriptionStats, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("getFeedStats: %w", err)
	}

	// Build query.
	query := query.Bool(
		query.BoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must be published within last month.
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(subscriptions)
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

	results, err := dataAPI.ItemsAggregation(ctx, query, len(subscriptions), aggs)
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
			slogctx.FromCtx(ctx).Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("subscription", subscriptions.GetByFeedID(feedID).GetID()))
			continue
		}

		stats[user.GetSubscriptions().GetByFeedID(feedID).GetID()] = SubscriptionStats{
			AvgDailyUpdates: float64(*updatesResult.Value),
		}
	}

	unreadCounts, err := getSubscriptionUnreadCounts(ctx, dataAPI, subscriptions)
	if err != nil {
		return nil, fmt.Errorf("unable to generate subscription stats: %w", err)
	}
	for feedID, unreadCount := range unreadCounts {
		if feedStats, found := stats[feedID]; found {
			feedStats.UnreadCount = int(unreadCount)
			stats[feedID] = feedStats
		}
	}

	return stats, nil
}

func getSubscriptionUnreadCounts(ctx context.Context, dataAPI DataAPI, subscriptionMetadata Subscriptions) (map[SubscriptionID]int64, error) {
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
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
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
		stats[user.GetSubscriptions().GetByFeedID(feedID).GetID()] = feed.DocCount
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
	err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.GetSubscriptions(),
	})
	if err != nil {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	return nil
}

func MatchRequestToFeed(ctx context.Context, dataAPI DataAPI, req *SubscriptionRequest) (*Feed, error) {
	// Find matches.
	feeds, _, err := dataAPI.SearchFeeds(ctx, query.Term("source_urls", req.GetURL()), 1, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to match request to feed: %w", err)
	}
	if len(feeds) == 0 {
		return nil, NewAPIError(ErrNotFound, http.StatusNotFound)
	}
	if len(feeds) != 1 {
		slogctx.FromCtx(ctx).Warn("More than one matching feed for request. Using first match.",
			slog.String("url", req.GetURL()),
			slog.String("matches", strings.Join(feeds.GetIDs(), ",")),
		)
	}
	return feeds[0], nil
}

func CreateFeed(ctx context.Context, dataAPI DataAPI, feed *Feed) error {
	err := dataAPI.CreateFeed(ctx, feed)
	if err != nil {
		return fmt.Errorf("unable to create feed: %w", err)
	}
	return nil
}

func FilterArticles(ctx context.Context, dataAPI DataAPI, filters *ListDisplayFilters, pagination Pagination) (Articles, Pagination, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve user: %w", err)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	subscriptions := user.GetSubscriptions()
	if len(filters.Subscriptions) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.Subscriptions...)
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, "", nil
	}
	query := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, filters.GetView(), subscriptions...)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := dataAPI.SearchItems(ctx, query, filters.GetCount(), &sort, &pagination)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}
	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, "", fmt.Errorf("could not generate articles from items: %w", err)
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
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
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

func FindSimilarArticles(ctx context.Context, dataAPI DataAPI, itemIDs ...ItemID) (Articles, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
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
				query.Should(BuildSubscriptionQueries(user, ViewUnread, user.GetSubscriptions()...)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	items, _, err := dataAPI.SearchItems(ctx, similarQuery, 15, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// GetSearchSuggestions will find suggestions for the global search from available subscriptions and articles.
func GetSearchSuggestions(ctx context.Context, dataAPI DataAPI, searchTerms string) (Subscriptions, Articles, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not fetch user: %w", err)
	}
	// Get article suggestions.
	feedIDs := user.GetSubscriptions().GetFeedIDs()
	itemsQuery := query.Bool(
		query.Filter(
			query.Terms("feed_id", feedIDs...),
		),
		query.Must(
			query.Bool(
				query.Should(
					query.SearchAsYouType(searchTerms, "title"),
					query.SearchAsYouType(searchTerms, "description"),
					query.SearchAsYouType(searchTerms, "categories"),
				),
			),
		),
	)
	itemResults, _, err := dataAPI.SearchItems(ctx, itemsQuery, 5, &SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get item matches: %w", err)
	}
	articles, err := GenerateArticles(ctx, itemResults)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error generating articles from items.", slog.Any("error", err))
	}

	// Generate subscriptions from data sources.
	metadataMatches := user.GetSubscriptions().Search(searchTerms)
	subscriptions, err := GetSubscriptions(ctx, dataAPI, metadataMatches.GetIDs()...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Error getting subscriptions.", slog.Any("error", err))
	}
	// Truncate subscription matches to 3 results.
	if len(subscriptions) > 3 {
		subscriptions = subscriptions[:3]
	}

	return subscriptions, articles, nil
}

// GetSearchResults will find results for the global search from available subscriptions and articles.
func GetSearchResults(ctx context.Context, dataAPI DataAPI, request *SearchRequest) (Subscriptions, []*Article, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not fetch user: %w", err)
	}
	itemResults, _, err := dataAPI.SearchItems(ctx, BuildSearchResultsQuery(user, request), 15, &SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get item matches: %w", err)
	}
	articles, err := GenerateArticles(ctx, itemResults)
	if err != nil {
		return nil, nil, fmt.Errorf("could not generate articles: %w", err)
	}
	// articles := make([]*partials.Article, 0, len(itemResults))
	// for article := range slices.Values(details) {
	// 	articles = append(articles, partials.NewArticleContent(article))
	// }

	// Generate subscriptions from data sources.
	subscriptions := make(Subscriptions, 0)
	metadataMatches := user.GetSubscriptions().Search(request.Text)
	if len(metadataMatches) > 0 {
		subscriptions, err := GetSubscriptions(ctx, dataAPI, metadataMatches.GetIDs()...)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Error getting subscriptions.", slog.Any("error", err))
		}
		// Truncate subscription matches to 3 results.
		if len(subscriptions) > 3 {
			subscriptions = subscriptions[:3]
		}
		// for s := range slices.Values(subscriptionMatches) {
		// 	subscriptions = append(subscriptions, partials.NewSubscriptionContent(s))
		// }
	}
	return subscriptions, articles, nil
}

func BuildItemsQuery(ctx context.Context, filters Filters, subscriptionIDs ...SubscriptionID) (query.Option, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to build items query: %w", err)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	subscriptions := user.GetSubscriptions().FilterByIDs(subscriptionIDs...)
	return query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, filters.GetView(), subscriptions...)...),
			),
		),
	), nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildSubscriptionQueries(user *User, view View, subscriptions ...*Subscription) []query.Option {
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

func BuildSearchResultsQuery(user *User, request *SearchRequest) query.Option {
	// var err error
	var loc *time.Location
	if request.Timezone != "" {
		loc, _ = time.LoadLocation(request.Timezone)
		// if err != nil {
		// 	slogctx.FromCtx(ctx).Debug("Error parsing timezone in request.",
		// 		slog.Any("error", err))
		// }
	} else {
		loc, _ = time.LoadLocation("UTC")
	}
	var since time.Time
	switch request.PublishedWithin {
	case SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
	case SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
	case SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
	case SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
	case SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
	}

	subscriptions := user.GetSubscriptions()
	if len(request.Subscriptions) > 0 {
		subscriptions = subscriptions.FilterByIDs(request.Subscriptions...)
	}

	return query.Bool(
		query.Filter(
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, request.View, subscriptions...)...),
			),
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
		// search suggestion).
		query.Must(
			// Search across title, description and content fields, with preference for match in that order (via field
			// boosting).
			query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// query.Bool(
			// 	query.Should(
			// 		query.MultiMatch(request.Text, "title", "description", "content"),
			// 		query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// 	),
			// ),
			query.SimpleQueryString(request.Categories, "", "categories"),
			query.SimpleQueryString(request.Authors, "", "authors", "contributors"),
		),
	)
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, subscription *Subscription) query.Option {
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
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *User, subscription *Subscription) query.Option {
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
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, subscription *Subscription) query.Option {
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
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.Customisation.ArticleFilters.Categories, "", "categories"),
		),
	)
}
