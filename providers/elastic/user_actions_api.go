// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var (
	ErrUserActionFailed      = errors.New("user action failed")
	ErrUserAlreadySubscribed = errors.New("user already subscribed")
)

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
