// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/views"
)

func getSubscriptions(ctx context.Context, api FeedsAPI, ids ...models.SubscriptionID) (models.Subscriptions, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}

	var states models.SubscriptionStates[models.SubscriptionID]

	if len(ids) == 0 {
		states = user.GetAllSubscriptionStates()
	} else {
		states = user.FilterSubscriptionStatesByID(ids...)
	}

	// Get customisation details for subscriptions.
	customisations, err := api.GetSubscriptions(ctx, ids...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Get feed data for subscriptions
	feedIDs := make([]models.FeedID, 0, len(states))
	for _, state := range states {
		feedIDs = append(feedIDs, state.GetFeedID())
	}
	feeds, err := api.GetFeeds(ctx, feedIDs...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Get unread counts.
	categoryCounts, err := getSubscriptionUnreadCounts(ctx, api, user.GetAllSubscriptionStatesByFeed())
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Generate subscriptions from data sources.
	var subscriptions []*models.Subscription
	for feed := range slices.Values(feeds) {
		var state *models.SubscriptionState
		for _, s := range states {
			if s.GetFeedID() == feed.GetID() {
				state = s
				break
			}
		}
		subscription, err := models.GenerateSubscription(feed,
			customisations.GetCustomisation(state.GetID()),
			state,
			categoryCounts.GetCount(feed.GetID()))
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

func filterSubscriptions(ctx context.Context, api FeedsAPI, filters *models.Filters) (models.Subscriptions, models.Pagination, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}
	states := user.GetAllSubscriptionStatesByFeed()

	var feedIDs []models.FeedID
	var subscriptionIDs []models.SubscriptionID

	var unreadCounts *aggregations.TermsAggregationResults

	switch filters.View {
	case models.ViewUnread:
		var err error
		// Get unread counts.
		unreadCounts, err = getSubscriptionUnreadCounts(ctx, api, states)
		if err != nil {
			return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
		}
		for _, state := range states {
			if !state.IsRead() && unreadCounts.GetCount(state.GetFeedID()) > 0 {
				feedIDs = append(feedIDs, state.GetFeedID())
				subscriptionIDs = append(subscriptionIDs, state.GetID())
			}
		}
	case models.ViewRead:
		for _, state := range states {
			if state.IsRead() {
				feedIDs = append(feedIDs, state.GetFeedID())
				subscriptionIDs = append(subscriptionIDs, state.GetID())
			}
		}
	case models.ViewAll:
		feedIDs = slices.Collect(maps.Keys(user.GetAllSubscriptionStatesByFeed()))
		for _, state := range states {
			subscriptionIDs = append(feedIDs, state.GetID())
		}
	}
	// Search subscription customisations for matches.
	customisationQuery := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.SubscriptionIDs(subscriptionIDs...),
			// Must match one of the given categories (if set).
			query.Categories(filters.Categories...),
		),
	)
	customisations, _, err := api.SearchSubscriptions(ctx, customisationQuery, len(subscriptionIDs), nil, nil)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	var customisationIDs []models.SubscriptionID
	for customisation := range slices.Values(customisations) {
		customisationIDs = append(customisationIDs, customisation.FeedID)
	}
	// Search feeds for matches.
	feedsQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(
					query.FeedIDs(customisationIDs...),
					query.Bool(
						query.Filter(
							query.FeedIDs(feedIDs...),
							query.Categories(filters.Categories...),
						),
					),
				),
			),
		),
	)
	sort := filters.Sort()
	feeds, pagination, err := api.SearchFeeds(ctx, feedsQuery, filters.CountAsInt(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Generate subscriptions from data sources.
	var subscriptions []*models.Subscription
	for feed := range slices.Values(feeds) {
		var state *models.SubscriptionState
		var count int
		for _, s := range states {
			if s.GetFeedID() == feed.GetID() {
				state = s
				break
			}
		}
		if unreadCounts != nil {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := models.GenerateSubscription(feed,
			customisations.GetCustomisation(state.GetID()),
			state,
			count,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, pagination, models.RespSuccess("Subscriptions fetched.")
}

func getSubscriptionUnreadCounts(ctx context.Context, api FeedsAPI, states models.SubscriptionStates[models.FeedID]) (*aggregations.TermsAggregationResults, error) {
	subscriptionQueries := make([]query.Option, 0, len(states))
	for _, state := range states {
		subscriptionQueries = append(subscriptionQueries, subscriptionQueryUnreadItems(state))
	}
	query := query.Bool(
		query.BoolQueryName("all_unread_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(slices.Collect(maps.Keys(states))...),
			// And should match one feed clause.
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	resp, err := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(states)))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve subscription category counts: %w", err)
	}
	var categoryCounts aggregations.TermsAggregationResults
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](resp.Aggregations, "UnreadCounts")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve subscription category counts: %w", err)
	}

	return &categoryCounts, nil
}

func filterArticles(ctx context.Context, api FeedsAPI, filters *models.Filters) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(slices.Collect(maps.Keys(user.GetAllSubscriptionStatesByFeed()))...),
			// Must match any of the given categories.
			query.Categories(filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, filters.View)...),
			),
		),
	)
	sort := filters.Sort()

	// Find items matching filters.
	items, pagination, err := api.SearchItems(ctx, query, filters.CountAsInt(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptions(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state, customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, pagination, models.RespSuccess("Fetched articles.")
}

func getArticles(ctx context.Context, api FeedsAPI, itemIDs ...models.ItemID) (models.Articles, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.ItemIDs(itemIDs...),
		),
	)

	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}

	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptions(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state, customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, models.RespSuccess("Fetched articles.")
}

func markArticles(ctx context.Context, api BackendAPI, mark models.Mark, itemIDs ...models.ItemID) *models.Response {
	return nil
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return models.RespInvalidUser()
	// }
	// if len(itemIDs) == 0 {
	// 	slogctx.FromCtx(ctx).Warn("Mark items requested but not items provided.")
	// 	return nil
	// }

	// articles, resp := getArticles(ctx, api, itemIDs...)
	// if resp.IsError() {
	// 	return resp
	// }
	// // Mark each item in the user data.
	// for feedID := range slices.Values(articles.GetItems().GetFeedIDs()) {
	// 	user.MarkItems(mark, feedID, articles.GetItems().FilterByFeed(feedID).GetIDs()...)
	// }
	// // Update the user object.
	// return api.UpdateUser(ctx, user.GetID(), map[string]any{
	// 	"subscriptions": user.Subscriptions,
	// 	"updated_at":    time.Now().UTC(),
	// })
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

func removeSubscriptions(ctx context.Context, api UserAPI, subscriptions ...models.SubscriptionID) *models.Response {
	return nil
	// if len(subscriptions) == 0 {
	// 	return nil
	// }
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return models.RespInvalidUser()
	// }
	// // Add the subscriptions to the user.
	// user.RemoveSubscriptions(subscriptions...)
	// // Update the user object.
	// return api.UpdateUser(ctx, user.GetID(), map[string]any{
	// 	"subscriptions": user.Subscriptions,
	// 	"updated_at":    time.Now().UTC(),
	// })
}

func markSubscriptions(ctx context.Context, api UserAPI, mark models.Mark, subscriptions ...models.SubscriptionID) *models.Response {
	return nil
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return models.RespInvalidUser()
	// }
	// // Mark subscriptions.
	// user.MarkSubscriptions(mark, subscriptions...)
	// slogctx.FromCtx(ctx).Debug("Marked subscriptions.",
	// 	slog.String("mark", string(mark)),
	// 	slog.String("subscriptions", strings.Join(subscriptions, ",")),
	// )
	// // Update the user object.
	// return api.UpdateUser(ctx, user.GetID(), map[string]any{
	// 	"subscriptions": user.Subscriptions,
	// 	"updated_at":    time.Now().UTC(),
	// })
}

// getHomePageData retrieves the data required to construct the home page content.
//
//nolint:funlen
func getHomePageData(ctx context.Context, api FeedsAPI) (*views.HomePageData, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}

	data := &views.HomePageData{
		Links: make(map[string]models.PageState),
	}

	// Generate links.
	data.Links["subscriptions"] = RestorePageState(ctx, "/home/subscriptions")
	data.Links["articles"] = RestorePageState(ctx, "/home/articles")

	// Get subscriptions.
	subscriptions, _, resp := filterSubscriptions(ctx, api, models.NewFilters())
	if resp.IsError() {
		return nil, resp
	}
	// Query definition for fetching unread items for all subscriptions.
	query := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(models.GetFeedIDs(slices.Values(subscriptions))...),
			// And should match one feed clause.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, models.ViewUnread)...),
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
	aggsResult, err := api.ItemsAggregation(ctx, query, aggs...)
	if err != nil {
		return nil, models.RespTemporaryIssue("Could not fetch data. Please try again.", err)
	}
	// Add the aggregations to the data
	data.Aggregations = aggsResult.Aggregations

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
	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	return nil, nil, models.RespInvalidUser()
	// }

	// subscriptionSearch := &query.MSearchOptions{
	// 	Query: query.Build(
	// 		query.Bool(
	// 			query.Filter(
	// 				query.Term("user_id", user.GetID()),
	// 			),
	// 			query.Must(
	// 				query.Match("title", searchTerms),
	// 				query.Match("description", searchTerms),
	// 				query.Match("categories", searchTerms),
	// 			),
	// 		),
	// 	),
	// 	Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	// }

	// itemsSearch := &query.MSearchOptions{
	// 	Query: query.Build(
	// 		query.Bool(
	// 			// query.Filter(
	// 			// 	query.FeedIDs(feedIDs...),
	// 			// ),
	// 			query.Must(
	// 				query.Match("title", searchTerms),
	// 				query.Match("description", searchTerms),
	// 				query.Match("categories", searchTerms),
	// 			),
	// 		),
	// 	),
	// 	Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	// }

	// subscriptions, articles, err := api.MultiSearch(ctx, subscriptionSearch, itemsSearch)
	// if err != nil {
	// 	return nil, nil, models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	// }

	return nil, nil, nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildSubscriptionQueries(user *models.User, view models.View) []query.Option {
	queries := make([]query.Option, 0, len(user.SubscriptionStates))
	// Work out what query to use based on the state filter.
	states := user.GetAllSubscriptionStates()
	switch view {
	case models.ViewRead:
		for _, state := range states {
			queries = append(queries, subscriptionQueryReadItems(user, state))
		}
	case models.ViewAll:
		for _, state := range states {
			queries = append(queries, subscriptionQueryAllItems(user, state))
		}
	case models.ViewUnread:
		fallthrough
	default:
		for _, state := range states {
			queries = append(queries, subscriptionQueryUnreadItems(state))
		}
	}
	return queries
}

// subscriptionQueryUnreadItems generates a query for finding unread items for the given subscription.
func subscriptionQueryUnreadItems(subscription *models.SubscriptionState) query.Option {
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
func subscriptionQueryReadItems(user *models.User, subscription *models.SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()

	switch {
	case subscription.GetMarkedRead().Equal(maxHistory):
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_match"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And be published/updated since the user max history.
				query.Bool(
					query.Should(
						query.Since("published", maxHistory),
						query.Since("updated", maxHistory),
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
						query.Between("published", maxHistory, subscription.GetMarkedRead()),
						query.Between("updated", maxHistory, subscription.GetMarkedRead()),
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
func subscriptionQueryAllItems(user *models.User, subscription *models.SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()

	return query.Bool(
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
