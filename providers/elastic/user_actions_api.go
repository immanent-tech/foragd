// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var (
	ErrUserActionFailed      = errors.New("user action failed")
	ErrUserAlreadySubscribed = errors.New("user already subscribed")
)

// GetUserSubscription retrieves the subscription with the given ID.
func (e *API) GetSubscription(ctx context.Context, subscriptionID models.SubscriptionID) (*models.Subscription, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespInvalidUser()
	}

	sub := user.GetSubscriptions().FindByID(subscriptionID)
	if sub == nil {
		return nil, &models.Response{
			StatusCode: http.StatusNoContent,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "No subscription with matching ID.",
			},
		}
	}
	feed, err := e.GetFeed(ctx, sub.GetFeedID())
	if err != nil {
		return nil, models.RespTemporaryIssue("The backend encountered an issue. Please retry.", err)
	}
	sub.Feed = feed
	return sub, nil
}

func (e *API) EditSubscription(ctx context.Context, subscriptionID models.SubscriptionID, edits *models.SubscriptionCustomisation) *models.Response {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	// Perform subscription edits.
	user.EditSubscription(subscriptionID, edits)
	// Save edits to user object.
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

func (e *API) GetSubscriptionsByID(ctx context.Context, filters models.Filters, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Subscriptions, models.Pagination, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}
	subscriptions := user.GetSubscriptions().FilterByID(subIDs...)

	// Get feeds matching subscriptions.
	feeds, err := e.GetAllFeeds(ctx, subscriptions.GetFeedIDs()...)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
	// Filter by feeds.
	subscriptions = subscriptions.FilterByFeed(feeds)
	// Add unread counts to feeds.
	err = e.GetSubscriptionUnreadCounts(ctx, subscriptions)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch subscriptions. Please try again.", err)
	}
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

// GetFeedUnreadCounts performs an aggregation over the items index to calculate
// unread counts for the given feed subscriptions.
func (e *API) GetSubscriptionUnreadCounts(ctx context.Context, subscriptions models.Subscriptions) error {
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
	resp, err := e.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(subscriptions)))
	if err != nil {
		return ErrFetchCtx
	}
	var categoryCounts aggregations.TermsAggregationResults
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](resp.Aggregations, "UnreadCounts")
	if err != nil {
		return ErrFetchCtx
	}
	for subscription := range slices.Values(subscriptions) {
		subscription.SetUnreadCount(categoryCounts.GetCount(subscription.GetFeedID()))
	}
	return nil
}

// AddSubscriptions will add Subscriptions to a User.
func (e *API) AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) *models.Response {
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
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// RemoveSubscriptions will remove the subscriptions for a user.
func (e *API) RemoveSubscriptions(ctx context.Context, subscriptions ...models.SubscriptionID) *models.Response {
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
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// UserActionMarkSubscriptions will mark user subscriptions with the given state.
func (e *API) MarkSubscriptions(ctx context.Context, mark models.Mark, subscriptions ...models.SubscriptionID) *models.Response {
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
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// GetItem retrieves the specified item with the given id and from the given
// feed. It checks for a subscription and will return false (without an error)
// if the current user is not subscribed.
func (e *API) GetArticle(ctx context.Context, itemID models.ItemID) (*models.Article, bool, *models.Response) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, false, &models.Response{
			StatusCode: http.StatusInternalServerError,
			UserMessage: &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Could not fetch article.",
			},
		}
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, false, models.RespInvalidUser()
	}

	req := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(
			query.Bool(
				query.Filter(
					// Must have the feedID and itemID
					// query.FeedIDs(feedID),
					query.ItemIDs(itemID),
					// Must be published or updated after the user max history.
					// query.Bool(
					// 	query.Should(
					// 		query.Since("published", user.GetMarkedRead(feedID)),
					// 		query.Since("updated", user.GetMarkedRead(feedID)),
					// 	),
					// ),
				),
			),
		),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, false, models.RespTemporaryIssue("Could not fetch article. Please try again.", err)
	}

	item, err := ExtractSource[*models.Item](res.Hits.Hits[0].Source_)
	if err != nil {
		return nil, false, models.RespTemporaryIssue("Could not fetch article. Please try again.", err)
	}

	if !user.IsSubscribed(item.GetFeedID()) {
		return nil, false, models.RespTemporaryIssue("Could not fetch article. Please try again.", err)
	}

	articles := models.GenerateArticles(user, item)

	return articles[0], true, models.RespSuccess("Fetched article.")
}

func (e *API) GetArticlesBySubscription(ctx context.Context, filters models.Filters, pagination models.Pagination, subIDs ...models.SubscriptionID) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespInvalidUser()
	}
	// Get subscriptions matching the filters.
	subscriptions := user.GetSubscriptions().FilterByID(subIDs...)

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

	// Search through items matching any given feeds filters, excluding any read
	// items.
	resp, err := e.ItemsSearch(ctx, query, filters, pagination)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch articles. Please try again.", err)
	}
	// Extract items and pagination values.
	items, lastSortValue, warnings := ExtractSourceFromHits[*models.Item](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}
	// Encode the pagination value.
	pagination, err = encodePagination(lastSortValue)
	if err != nil {
		return nil, "", models.RespTemporaryIssue("Could not fetch article. Please try again.", err)
	}
	// Create articles from the items.
	articles := models.GenerateArticles(user, items...)

	return articles, pagination, models.RespSuccess("Fetched articles.")
}

// GetItemsByID fetches the items with the given IDs.
func (e *API) GetArticlesByID(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrFetchCtx
	}

	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(query.ItemIDs(itemIDs...)),
		WithSearchSize(len(itemIDs)),
	).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	slogctx.FromCtx(ctx).Debug("Searched items.",
		slog.Int64("hits", resp.Hits.Total.Value))

	items, _, err := ExtractSourceFromHits[*models.Item](resp.Hits.Hits)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	articles := models.GenerateArticles(user, items...)

	return articles, nil
}

// MarkItems will mark the given items for the given feeds with the given state for the user.
func (e *API) MarkItems(ctx context.Context, mark models.Mark, itemIDs ...models.ItemID) *models.Response {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespInvalidUser()
	}
	if len(itemIDs) == 0 {
		slogctx.FromCtx(ctx).Warn("Mark items requested but not items provided.")
		return nil
	}
	// Get item details.
	articles, err := e.GetArticlesByID(ctx, itemIDs...)
	if err != nil {
		return models.RespTemporaryIssue("Could not perform action. Please try again.", err)
	}
	// Mark each item in the user data.
	for feedID := range slices.Values(articles.GetItems().GetFeedIDs()) {
		user.MarkItems(mark, feedID, articles.GetItems().FilterByFeed(feedID).GetIDs()...)
	}
	// Update the user object.
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// GetTopItemCategories gets the top 10 most-used categories across the list of feeds.
func (e *API) GetTopItemCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, *models.Response) {
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(feeds...),
		),
	)
	resp, err := e.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("TopCategories", "categories.raw", 10))
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
