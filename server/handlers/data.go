// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/views"
)

// getAllSubscriptions retrieves all the users subscriptions.
func getAllSubscriptions(ctx context.Context, api FeedsAPI) (models.Subscriptions, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}
	// Get user subscriptions.
	subscriptions := user.GetSubscriptions()

	// Get feeds matching subscriptions.
	feeds, err := api.GetFeeds(ctx, subscriptions.GetFeedIDs()...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Filter by feeds.
	subscriptions = subscriptions.FilterByFeed(feeds)
	// Add unread counts to feeds.
	if resp := setSubscriptionUnreadCounts(ctx, api, subscriptions); resp.IsError() {
		return subscriptions, resp
	}
	// Filter subscriptions with given filters.
	subscriptions = subscriptions.Sort(
		models.Sort{
			SortBy:    models.SortByUnreadCount,
			SortOrder: models.SortOrderDesc,
		})

	return subscriptions, models.RespSuccess("Subscriptions fetched.")
}

// getFilteredSubscriptions retrieves filtered user subscriptions, and potentially paginated.
func getFilteredSubscriptions(ctx context.Context, api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Subscriptions, models.Pagination, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}
	subscriptions := user.GetSubscriptions().FilterByID(subIDs...)

	// Get feeds matching subscriptions.
	feeds, err := api.GetFeeds(ctx, subscriptions.GetFeedIDs()...)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Filter by feeds.
	subscriptions = subscriptions.FilterByFeed(feeds)
	// Add unread counts to feeds.
	if resp := setSubscriptionUnreadCounts(ctx, api, subscriptions); resp.IsError() {
		return subscriptions, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}

	filters := models.FiltersFromCtx(ctx)

	// Filter subscriptions with given filters.
	subscriptions = subscriptions.
		FilterByCategory(filters.Categories...).
		FilterByView(filters.View).
		Sort(filters.Sort())
	// Generate pagination.
	from, err := strconv.Atoi(pagination)
	if err != nil {
		from = 0
	}
	to := min(from+filters.CountAsInt(), len(subscriptions))
	pagination = strconv.Itoa(to)
	return subscriptions[from:to], pagination, models.RespSuccess("Subscriptions fetched.")
}

func setSubscriptionUnreadCounts(ctx context.Context, api FeedsAPI, subscriptions models.Subscriptions) *models.Response {
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		subscriptionQueries = append(subscriptionQueries, subscriptionQueryUnreadItems(subscription))
	}
	query := query.Bool(
		query.BoolQueryName("all_unread_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	resp, err := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(subscriptions)))
	if err != nil {
		return models.RespNonCriticalError("Subscription unread counts could not be retrieved.", err)
	}
	var categoryCounts aggregations.TermsAggregationResults
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](resp.Aggregations, "UnreadCounts")
	if err != nil {
		return models.RespNonCriticalError("Subscription unread counts could not be retrieved.", err)
	}
	for subscription := range slices.Values(subscriptions) {
		subscription.SetUnreadCount(categoryCounts.GetCount(subscription.GetFeedID()))
	}
	return nil
}

func searchArticles(ctx context.Context, api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}
	// Get subscriptions matching the filters.
	subscriptions := user.GetSubscriptions().FilterByID(subIDs...)
	filters := models.FiltersFromCtx(ctx)

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Categories(filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(subscriptions, filters.View)...),
			),
		),
	)
	sort := filters.Sort()

	items, pagination, err := api.SearchItems(ctx, query, filters.CountAsInt(), &sort, &pagination)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	// Create articles from the items.
	articles := models.ConvertItemsToArticles(user, items...)

	return articles, pagination, models.RespSuccess("Fetched articles.")
}

func getArticles(ctx context.Context, api FeedsAPI, itemIDs ...models.ItemID) (models.Articles, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}
	// Get subscriptions matching the filters.
	subscriptions := user.GetSubscriptions()

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// Must match any of the given item IDs,
			query.ItemIDs(itemIDs...),
		),
	)

	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	// Create articles from the items.
	articles := models.ConvertItemsToArticles(user, items...)

	return articles, models.RespSuccess("Fetched articles.")
}

func markArticles(ctx context.Context, api BackendAPI, mark models.Mark, itemIDs ...models.ItemID) *models.Response {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	if len(itemIDs) == 0 {
		slogctx.FromCtx(ctx).Warn("Mark items requested but not items provided.")
		return nil
	}

	articles, resp := getArticles(ctx, api, itemIDs...)
	if resp.IsError() {
		return resp
	}
	// Mark each item in the user data.
	for feedID := range slices.Values(articles.GetItems().GetFeedIDs()) {
		user.MarkItems(mark, feedID, articles.GetItems().FilterByFeed(feedID).GetIDs()...)
	}
	// Update the user object.
	return api.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

func getItemTopCategories(ctx context.Context, api FeedsAPI, feeds ...models.FeedID) ([]models.Category, *models.Response) {
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(feeds...),
		),
	)
	resp, err := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("TopCategories", "categories.raw", 10))
	if err != nil {
		return nil, &models.Response{
			StatusCode:    http.StatusNoContent,
			InternalError: err,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Could not retrieve categories.",
			},
		}
	}
	var topCategories aggregations.TermsAggregationResults
	topCategories.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](resp.Aggregations, "TopCategories")
	if err != nil {
		return nil, &models.Response{
			StatusCode:    http.StatusNoContent,
			InternalError: err,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Could not retrieve categories.",
			},
		}
	}

	return topCategories.BucketNames(), models.RespSuccess("Retrieved categories.")
}

func addSubscriptions(ctx context.Context, api UserAPI, subscriptions models.Subscriptions) *models.Response {
	if len(subscriptions) == 0 {
		return nil
	}
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	// Add the subscriptions to the user.
	user.AddSubscriptions(subscriptions)
	// Update the user object.
	return api.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

func removeSubscriptions(ctx context.Context, api UserAPI, subscriptions ...models.SubscriptionID) *models.Response {
	if len(subscriptions) == 0 {
		return nil
	}
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	// Add the subscriptions to the user.
	user.RemoveSubscriptions(subscriptions...)
	// Update the user object.
	return api.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

func markSubscriptions(ctx context.Context, api UserAPI, mark models.Mark, subscriptions ...models.SubscriptionID) *models.Response {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	// Mark subscriptions.
	user.MarkSubscriptions(mark, subscriptions...)
	slogctx.FromCtx(ctx).Debug("Marked subscriptions.",
		slog.String("mark", string(mark)),
		slog.String("subscriptions", strings.Join(subscriptions, ",")),
	)
	// Update the user object.
	return api.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// getHomePageData retrieves the data required to construct the home page content.
//
//nolint:funlen
func getHomePageData(ctx context.Context, api FeedsAPI) (*views.HomePageData, *models.Response) {
	data := &views.HomePageData{
		Links: make(map[string]models.PageState),
	}

	// Generate links.
	data.Links["subscriptions"] = RestorePageState(ctx, "/home/subscriptions")
	data.Links["articles"] = RestorePageState(ctx, "/home/articles")

	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}
	// Get subscriptions.
	subscriptions := user.GetSubscriptions()
	// Query definition for fetching unread items for all subscriptions.
	query := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(subscriptions, models.ViewUnread)...),
			),
		),
	)

	var aggs []aggregations.Aggregation
	TermsField := "categories.raw"

	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	SampleField := "feed_id"
	DefaultMaxDocsPerValue := 10
	ShardSize := 1000
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "top_categories_sample",
			Definition: types.Aggregations{
				DiversifiedSampler: &types.DiversifiedSamplerAggregation{
					Field:           &SampleField,
					MaxDocsPerValue: &DefaultMaxDocsPerValue,
					ShardSize:       &ShardSize,
				},
				Aggregations: map[string]types.Aggregations{
					"top_categories": {
						Terms: &types.TermsAggregation{
							Field: &TermsField,
						},
					},
				},
			},
		},
	)
	// Aggregation definition for fetching the rare item categories across all subscriptions.
	MaxDocCount := int64(5)
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "rare_categories",
			Definition: types.Aggregations{
				RareTerms: &types.RareTermsAggregation{
					Field:       &TermsField,
					MaxDocCount: &MaxDocCount,
				},
			},
		},
	)

	// Aggregation definition for fetching a set of random items from all subscriptions.
	ItemIDField := "item_id"
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "random_items",
			Definition: types.Aggregations{
				RandomSampler: &types.RandomSamplerAggregation{
					Probability: 0.1,
				},
				Aggregations: map[string]types.Aggregations{
					"items": {
						Terms: &types.TermsAggregation{
							Field: &ItemIDField,
						},
					},
				},
			},
		},
	)

	// Perform the request.
	resp, err := api.ItemsAggregation(ctx, query, aggs...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch data. Please try again.", err)
	}
	// Add the aggregations to the data
	data.Aggregations = resp.Aggregations

	return data, nil
}

// getHomePageArticles retrieves a list of articles to display on the home page along with other content.
func getHomePageArticles(ctx context.Context, api FeedsAPI, data *views.HomePageData) (models.Articles, *models.Response) {
	// Get the rare categories aggregation.
	randomItemsAgg, err := aggregations.ExtractAggregation[map[string]any](data.Aggregations, "random_items")
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// itemsAgg, err := aggregations.ExtractAggregation[*types.StringTermsAggregate](randomItemsAgg, "sterms#items")
	itemsAgg, ok := randomItemsAgg["sterms#items"].(map[string]any)
	if !ok {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// if err != nil {
	// 	return nil, fmt.Errorf("could not get random items: %w", err)
	// }
	buckets, ok := itemsAgg["buckets"].([]any)
	if !ok {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	itemIDs := make([]models.ItemID, 0, len(buckets))
	for bucket := range slices.Values(buckets) {
		value, ok := bucket.(map[string]any)
		if !ok {
			continue
		}
		key, ok := value["key"].(string)
		if !ok {
			continue
		}
		itemIDs = append(itemIDs, key)
	}

	articles, resp := getArticles(ctx, api, itemIDs...)
	if resp.IsError() {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	return articles, nil
}

func getSearchSuggestions(ctx context.Context, api FeedsAPI, searchTerms string) (models.Subscriptions, models.Articles, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, models.RespInvalidUser()
	}
	subscriptions := user.GetSubscriptions()

	feedsSearch := &elastic.MSearchOptions{
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.FeedIDs(subscriptions.GetFeedIDs()...),
				),
				query.Must(
					query.Match("title", searchTerms),
					query.Match("description", searchTerms),
					query.Match("categories", searchTerms),
				),
			),
		),
		Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	}

	itemsSearch := &elastic.MSearchOptions{
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.FeedIDs(subscriptions.GetFeedIDs()...),
				),
				query.Must(
					query.Match("title", searchTerms),
					query.Match("description", searchTerms),
					query.Match("categories", searchTerms),
				),
			),
		),
		Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	}

	feeds, items, err := api.MultiSearch(ctx, feedsSearch, itemsSearch)
	if err != nil {
		return nil, nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	return models.ConvertFeedsToSubscriptions(user, feeds...), models.ConvertItemsToArticles(user, items...), nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildSubscriptionQueries(subscriptions models.Subscriptions, view models.View) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	switch view {
	case models.ViewRead:
		for subscription := range slices.Values(subscriptions) {
			queries = append(queries, subscriptionQueryReadItems(subscription))
		}
	case models.ViewAll:
		for subscription := range slices.Values(subscriptions) {
			queries = append(queries, subscriptionQueryAllItems(subscription))
		}
	case models.ViewUnread:
		fallthrough
	default:
		for subscription := range slices.Values(subscriptions) {
			queries = append(queries, subscriptionQueryUnreadItems(subscription))
		}
	}
	return queries
}

// subscriptionQueryUnreadItems generates a query for finding unread items for the given subscription.
func subscriptionQueryUnreadItems(subscription *models.Subscription) query.Option {
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_query_unread"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And should be newer than last read or explicitly marked unread.
			query.Bool(
				query.Should(
					query.Since("published", subscription.GetMarkedRead()),
					query.Since("updated", subscription.GetMarkedRead()),
					query.ItemIDs(subscription.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.ItemIDs(subscription.GetReadItems()...),
				),
			),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding read items for the given subscription.
func subscriptionQueryReadItems(subscription *models.Subscription) query.Option {
	switch {
	case subscription.GetMarkedRead().Equal(subscription.GetMaxHistory()):
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_match"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And be published/updated since the user max history.
				query.Bool(
					query.Should(
						query.Since("published", subscription.GetMaxHistory()),
						query.Since("updated", subscription.GetMaxHistory()),
						query.ItemIDs(subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.ItemIDs(subscription.GetUnreadItems()...),
					),
				),
			),
		)
	default:
		return query.Bool(
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And should be between the user max history and last read time.
				query.Bool(
					query.Should(
						query.Between("published", subscription.GetMaxHistory(), subscription.GetMarkedRead()),
						query.Between("updated", subscription.GetMaxHistory(), subscription.GetMarkedRead()),
						query.ItemIDs(subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.ItemIDs(subscription.GetUnreadItems()...),
					),
				),
			),
		)
	}
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func subscriptionQueryAllItems(subscription *models.Subscription) query.Option {
	return query.Bool(
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", subscription.GetMaxHistory()),
					query.Since("updated", subscription.GetMaxHistory()),
				),
			),
		),
	)
}

// encodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a models.Pagination object, which is
// safe for use in API query parameters.
func encodePagination(sortValues []types.FieldValue) (models.Pagination, error) {
	if len(sortValues) == 0 {
		return "", nil
	}
	// Marshal sort values into json.
	data, err := json.Marshal(sortValues)
	if err != nil {
		return "", errors.Join(elastic.ErrPagination, fmt.Errorf("could not encode pagination values: %w", err))
	}
	// Return as HTML encoded string.
	return url.QueryEscape(string(data)), nil
}

// decodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func decodePagination(pagination models.Pagination) ([]types.FieldValue, error) {
	if pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(pagination)
	if err != nil {
		return nil, errors.Join(elastic.ErrPagination, fmt.Errorf("could not decode pagination values: %w", err))
	}
	// Unmarshal sort values.
	var sortValues []types.FieldValue
	err = json.Unmarshal([]byte(data), &sortValues)
	if err != nil {
		return nil, errors.Join(elastic.ErrPagination, fmt.Errorf("could not decode pagination values: %w", err))
	}
	// Return sort values.
	return sortValues, nil
}
