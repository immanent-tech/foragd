// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
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

// UserActionMarkItemsRead will mark the given items with the given state for the user.
func (e *API) MarkItems(ctx context.Context, marks ...*models.MarkFeedItems) error {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}
	// Mark items for the given feeds.
	for mark := range slices.Values(marks) {
		user.MarkItems(mark.Mark, mark.Feed, mark.Items...)
	}
	// Update the user object.
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// GetItem retrieves the specified item with the given id and from the given
// feed. It checks for a subscription and will return false (without an error)
// if the current user is not subscribed.
func (e *API) GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, false, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, false, ErrNoUserCtx
	}

	if !user.IsSubscribed(feedID) {
		return nil, false, ErrNoUserCtx
	}

	req := NewSearchRequest(e.GetAPI(),
		WithSearchIndex(index),
		WithSearchQueryOptions(
			query.Bool(
				query.Filter(
					// Must have the feedID and itemID
					query.FeedIDs(feedID),
					query.ItemIDs(itemID),
					// Must be published or updated after the user max history.
					query.Bool(
						query.Should(
							query.Since("published", user.GetMarkedRead(feedID)),
							query.Since("updated", user.GetMarkedRead(feedID)),
						),
					),
				),
			),
		),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, false, errors.Join(ErrSearchFailed, err)
	}

	item, err := ExtractSource[*models.Item](res.Hits.Hits[0].Source_)
	if err != nil {
		return nil, false, errors.Join(ErrSearchFailed, err)
	}

	return item, true, nil
}

// UserGetItems will search Elasticsearch for unread items (with
// given filters applied) for the given user, and, returns the items as well as
// pagination details for paging through the results.
func (e *API) GetItems(ctx context.Context) (models.Items, models.Pagination, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", ErrFetchCtx
	}
	filters := models.FiltersFromCtx(ctx)
	// Get subscriptions matching the filters.
	subscriptions := user.GetSubscriptions().
		FilterByFeedID(filters.Feeds...).
		FilterByCategory(filters.Categories...)

	// Work out what query to use based on the state filter.
	query := generateItemsQueryClause(filters.View, subscriptions)

	// Search through items matching any given feeds filters, excluding any read
	// items.
	resp, err := e.ItemsSearch(ctx, query, filters)
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}
	// Extract items and pagination values.
	items, lastSortValue, warnings := ExtractSourceFromHits[*models.Item](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}
	// Encode the pagination value.
	pagination, err := encodePagination(lastSortValue)
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}

	// Decorate items with user state.
	subscriptionsByFeed := subscriptions.ByFeed()
	for item := range slices.Values(items) {
		// Add the state for the item from the user object, to the item object.
		itemState := subscriptionsByFeed[item.GetFeedID()].GetItemState(item.GetID())
		item.SetUserItemState(itemState)
	}

	return items, pagination, nil
}

// GetUserSubscriptions returns all subscriptions for a user with feed and state details added.
func (e *API) GetSubscriptions(ctx context.Context) (models.Subscriptions, models.Pagination, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.NewMessage(
			"Could not fetch subscriptions.",
			models.MessageStatusError,
			models.WithError(ErrNoUserCtx))
	}
	subscriptions := user.GetSubscriptions()
	// Get feeds matching subscriptions.
	feeds, err := e.GetAllFeeds(ctx, subscriptions.GetFeedIDs()...)
	if err != nil {
		return nil, "", models.NewMessage(
			"Could not fetch subscriptions.",
			models.MessageStatusError,
			models.WithError(err))
	}
	// Filter by feeds.
	subscriptions = subscriptions.FilterByFeed(feeds)
	// Add unread counts to feeds.
	err = e.GetSubscriptionUnreadCounts(ctx, subscriptions)
	if err != nil {
		return nil, "", models.NewMessage(
			"Could not fetch subscription unread counts",
			models.MessageStatusWarning,
			models.WithError(err))
	}
	subscriptions = subscriptions.Filter(models.FiltersFromCtx(ctx))
	// Generate pagination values.
	pagination := models.FiltersFromCtx(ctx).Pagination
	from, err := strconv.Atoi(pagination)
	if err != nil {
		from = 0
	}
	to := min(from+models.FiltersFromCtx(ctx).Count, len(subscriptions))
	pagination = strconv.Itoa(to)
	spew.Dump(from, to)
	return subscriptions[from:to], pagination, nil
}

// GetFeedUnreadCounts performs an aggregation over the items index to calculate
// unread counts for the given feed subscriptions.
func (e *API) GetSubscriptionUnreadCounts(ctx context.Context, subscriptions models.Subscriptions) error {
	countResults, err := e.ItemsAggregation(ctx, unreadFeedItemsQuery(subscriptions), NewTermsAggregation("UnreadCounts", "feed_id", len(subscriptions)))
	if err != nil {
		return ErrFetchCtx
	}
	var categoryCounts TermsAggregationResults
	categoryCounts.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](countResults, "UnreadCounts")
	if err != nil {
		return ErrFetchCtx
	}
	for subscription := range slices.Values(subscriptions) {
		subscription.SetUnreadCount(categoryCounts.GetCount(subscription.GetFeedID()))
	}
	return nil
}

// AddSubscriptions will add Subscriptions to a User.
func (e *API) AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error {
	if len(subscriptions) == 0 {
		return nil
	}
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.NewMessage(
			"Unable to add subscriptions.",
			models.MessageStatusError,
			models.WithError(ErrNoUserCtx))
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
func (e *API) MarkSubscriptions(ctx context.Context, marks *models.MarkFeeds) error {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}
	// Mark subscriptions.
	user.MarkSubscriptions(marks.Mark, marks.Feeds...)
	// Update the user object.
	return e.UpdateUser(ctx, user.GetID(), map[string]any{
		"subscriptions": user.Subscriptions,
		"updated_at":    time.Now().UTC(),
	})
}

// generateItemsQueryClause selects the appropriate query clause for retrieving
// items using the given filters.
func generateItemsQueryClause(view models.View, subscriptions models.Subscriptions) query.Option {
	// Work out what query to use based on the state filter.
	switch view {
	case models.ViewRead:
		return readFeedItemsQuery(subscriptions)
	case models.ViewUnread:
		return unreadFeedItemsQuery(subscriptions)
	case models.ViewAll:
		return allFeedItemsQuery(subscriptions)
	default:
		return unreadFeedItemsQuery(subscriptions)
	}
}

// unreadFeedItemsQuery generates a query for matching unread items using the
// given filters. An optional duration can be specified, which will further
// restrict the match to items published since the current time minus the
// duration.
func unreadFeedItemsQuery(subscriptions models.Subscriptions) query.Option {
	clauses := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		clauses = append(clauses,
			query.Bool(
				query.BoolQueryName(subscription.GetFeedID()+"_match"),
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
			),
		)
	}

	return query.Bool(
		query.BoolQueryName("unread_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// query.Categories(subscriptions.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
	)
}

// readFeedItemsQuery generates a query for matching read items using the given filters.
func readFeedItemsQuery(subscriptions models.Subscriptions) query.Option {
	clauses := make([]query.Option, 0, len(subscriptions))

	for subscription := range slices.Values(subscriptions) {
		// Ignore feed if user has never marked it as read.
		if subscription.GetMarkedRead() == subscription.GetMaxHistory() {
			clauses = append(clauses,
				query.Bool(
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
				),
			)
		} else {
			clauses = append(clauses,
				query.Bool(
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
				),
			)
		}
	}

	return query.Bool(
		query.BoolQueryName("read_items"),
		query.Filter(
			// Must match any of the given feed IDs
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// query.Categories(subscriptions.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
	)
}

func allFeedItemsQuery(subscriptions models.Subscriptions) query.Option {
	clauses := make([]query.Option, 0, len(subscriptions))

	for subscription := range slices.Values(subscriptions) {
		clauses = append(clauses,
			query.Bool(
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
			),
		)
	}

	return query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(subscriptions.GetFeedIDs()...),
			// query.Categories(subscriptions.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
	)
}

// // unreadFeedItemsQuery generates a query for matching unread items using the given filters.
// func newItemsQuery(user *models.User, filters models.Filters, since time.Duration) query.Option {
// 	cutoff := time.Now().Add(since)

// 	clauses := make([]query.Option, 0, len(filters.GetFeeds()))
// 	for _, id := range filters.GetFeeds() {
// 		clauses = append(clauses,
// 			query.Bool(
// 				query.Filter(
// 					// Must match this feed.
// 					query.Term("feed_id", id),
// 					// And should be newer than last read or explicitly marked unread.
// 					query.Bool(
// 						query.Should(
// 							query.Since("publishedParsed", cutoff),
// 							query.Since("updatedParsed", cutoff),
// 							query.ItemIDs(user.GetItemIDsWithState(models.Unread, id)...),
// 						),
// 					),
// 				),
// 			),
// 		)
// 	}

// 	return query.Bool(
// 		query.Filter(
// 			// Must match any of the given feed IDs.
// 			query.FeedIDs(filters.GetFeeds()...),
// 			query.Categories(filters.GetCategories()...),
// 			// And should match one feed clause.
// 			query.Bool(
// 				query.Should(clauses...),
// 			),
// 		),
// 		query.MustNot(
// 			// Must not match any read item IDs.
// 			query.ItemIDs(user.GetItemIDsWithState(models.Read, filters.GetFeeds()...)...),
// 		),
// 	)
// }
