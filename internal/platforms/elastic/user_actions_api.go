// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/query"
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
							query.Since("publishedParsed", user.GetMarkedRead(feedID)),
							query.Since("updatedParsed", user.GetMarkedRead(feedID)),
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
func (e *API) GetItems(ctx context.Context, filters models.Filters) (models.Items, models.Pagination, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.WrapError(ErrNoUserCtx, "elastic", "get items failed")
	}

	// Get subscriptions matching the filters.
	subscriptions := user.GetSubscriptions().
		FilterByFeedID(filters.Feeds...).
		FilterByCategory(filters.Categories...)

	// Work out what query to use based on the state filter.
	query := generateItemsQueryClause(filters.View, user, subscriptions)

	// Search through items matching any given feeds filters, excluding any read
	// items.
	resp, err := e.ItemsSearch(ctx, query, filters, "")
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}
	// Extract items and pagination values.
	items, lastSortValue, warnings := ExtractSourceFromHits[*models.Item](resp.Hits.Hits)
	if warnings != nil {
		logging.FromContext(ctx).Warn("Problems occurred while extracting source from docs.",
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

// GetSubscriptions will search Elasticsearch for subscribed feeds (with
// given filters applied) for the given user, and, returns the feeds.
func (e *API) GetSubscriptions(ctx context.Context, filters models.Filters) (models.Subscriptions, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.NewMessage(
			"Could not fetch subscriptions.",
			models.MessageStatusError,
			models.WithError(ErrNoUserCtx))
	}
	subscriptions := user.GetSubscriptions().FilterByFeedID(filters.Feeds...)
	categories := filters.Categories
	// Adjust the feed filters.
	filters.Feeds = subscriptions.GetFeedIDs()
	filters.Categories = nil
	// Get feeds matching subscriptions
	feeds, err := e.FeedsSearch(ctx, filters, "")
	if err != nil {
		return nil, models.NewMessage(
			"Could not fetch subscriptions.",
			models.MessageStatusError,
			models.WithError(err))
	}
	// Attach feeds to subscriptions.
	for feed := range slices.Values(feeds) {
		idx := slices.IndexFunc(subscriptions, func(v *models.Subscription) bool {
			return v.GetFeedID() == feed.GetID()
		})
		if idx != -1 {
			subscriptions[idx].Feed = feed
		}
	}
	subscriptions = subscriptions.FilterByCategory(categories...)

	// Add unread counts to feeds.
	err = e.GetSubscriptionUnreadCounts(ctx, subscriptions)
	if err != nil {
		return nil, models.NewMessage(
			"Could not fetch subscription unread counts",
			models.MessageStatusWarning,
			models.WithError(err))
	}

	// Filter the feeds by view filter.
	if filters.View == models.ViewRead {
		subscriptions = subscriptions.FilterByRead()
	}
	if filters.View == models.ViewUnread {
		subscriptions = subscriptions.FilterByUnread()
	}

	// If the sort_by filters is unread count, sort the list of feeds by user
	// unread count. We can't do this in Elasticsearch as the unread count comes
	// from an aggregation and is not a field on the feed documents.
	if filters.Sort().SortBy == models.SortByUnreadCount {
		slices.SortFunc(subscriptions, models.CompareSubscriptionUnreadCount)
		if filters.Sort().SortOrder == models.SortOrderDesc {
			slices.Reverse(subscriptions)
		}
	}

	return subscriptions, nil
}

// GetFeedUnreadCounts performs an aggregation over the items index to calculate
// unread counts for the given feed subscriptions.
func (e *API) GetSubscriptionUnreadCounts(ctx context.Context, subscriptions models.Subscriptions) error {
	countResults, err := e.ItemsAggregation(ctx, unreadFeedItemsQuery(subscriptions), NewTermsAggregation("UnreadCounts", "feed_id"))
	if err != nil {
		return models.WrapError(err, "elastic", "get feed unread counts failed")
	}
	var categoryCounts TermsAggregationResults
	categoryCounts.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](countResults, "UnreadCounts")
	if err != nil {
		return models.WrapError(err, "elastic", "get feed unread counts failed")
	}
	unreadCounts := make(map[models.FeedID]int)
	for feedID := range slices.Values(subscriptions.GetFeedIDs()) {
		unreadCounts[feedID] = categoryCounts.GetCount(feedID)
	}

	subscriptions.UpdateUnreadCounts(unreadCounts)

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

// // UserActionCountUnread will return a total count of unread items across all
// // feeds (with the given filters applied) for the user.
// func UserActionCountUnread(ctx context.Context, esapi *typedmodels.API, filters models.Filters) (int64, error) {
// 	user, found := models.UserFromCtx(ctx)
// 	if !found {
// 		return 0, ErrGetUserFailed
// 	}

// 	filters.SetFeeds(filterSubscribedFeeds(user, filters)...)

// 	// Get the unread counts for the feeds.
// 	resp, err := ItemsCount(ctx, esapi, unreadFeedItemsQuery(user, filters))
// 	if err != nil {
// 		return 0, errors.Join(ErrUserActionFailed, err)
// 	}

// 	return resp.Count, nil
// }

// func (c *Client) UserActionGetFeed(ctx context.Context, esapi *typedmodels.API, feedID models.FeedID) (*models.APIFeed, error) {
// 	user, found := models.UserFromCtx(ctx)
// 	if !found {
// 		return nil, ErrGetUserFailed
// 	}

// 	if !user.IsSubscribed(feedID) {
// 		return nil, errors.Join(ErrUserActionFailed, ErrNoHits)
// 	}

// 	// Get the feed.
// 	feed, err := c.GetFeedByID(ctx, feedID)
// 	if err != nil {
// 		return nil, errors.Join(ErrUserActionFailed, err)
// 	}

// 	query := query.Bool(
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", feedID),
// 			// Must not match any read item IDs.
// 			query.ItemIDs(user.GetItemIDsWithState(models.Read, feedID)...),
// 			// And should be newer than last read or explicitly marked unread.
// 			query.Bool(
// 				query.Should(
// 					query.Since("publishedParsed", user.GetFeedLastRead(feedID)),
// 					query.Since("updatedParsed", user.GetFeedLastRead(feedID)),
// 					query.ItemIDs(user.GetItemIDsWithState(models.Unread, feedID)...),
// 				),
// 			),
// 		),
// 	)

// 	resp, err := ItemsCount(ctx, esapi, query)
// 	if err != nil {
// 		return nil, errors.Join(ErrUserActionFailed, err)
// 	}

// 	// Add user data to feed.
// 	addUserDataToFeed(user, feed, int(resp.Count))

// 	return feed, nil
// }

// func UserActionGetItemCategories(ctx context.Context, esapi *typedmodels.API, filters models.Filters) ([]models.CategoryCount, error) {
// 	user, found := models.UserFromCtx(ctx)
// 	if !found {
// 		return nil, ErrGetUserFailed
// 	}

// 	// Unset the category filter.
// 	filters.Categories = nil

// 	resp, err := ItemsAggregation(ctx, esapi, generateItemsQueryClause(user, filters), NewTermsAggregation("categories", "categories.raw"))
// 	if err != nil {
// 		return nil, errors.Join(ErrUserActionFailed, err)
// 	}

// 	var results TermsAggregationResults

// 	results.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](resp, "categories")
// 	if err != nil {
// 		return nil, errors.Join(ErrUserActionFailed, err)
// 	}

// 	categories := make([]models.CategoryCount, 0, results.BucketCount())

// 	for _, category := range results.BucketNames() {
// 		categories = append(categories, models.CategoryCount{Category: category, Count: results.GetCount(category)})
// 	}

// 	return categories, nil
// }

// generateItemsQueryClause selects the appropriate query clause for retrieving
// items using the given filters.
func generateItemsQueryClause(view models.View, user *models.User, subscriptions models.Subscriptions) query.Option {
	// Work out what query to use based on the state filter.
	switch view {
	case models.ViewRead:
		return readFeedItemsQuery(user, subscriptions)
	case models.ViewUnread:
		return unreadFeedItemsQuery(subscriptions)
	case models.ViewAll:
		return allFeedItemsQuery(user, subscriptions)
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
func readFeedItemsQuery(user *models.User, subscriptions models.Subscriptions) query.Option {
	clauses := make([]query.Option, 0, len(subscriptions))

	for feedID, info := range subscriptions.ByFeed() {
		// Ignore feed if user has never marked it as read.
		if info.GetMarkedRead() == user.GetMaxHistory() {
			clauses = append(clauses,
				query.Bool(
					query.BoolQueryName(feedID+"_match"),
					query.Filter(
						// Must match this feed.
						query.Term("feed_id", feedID),
						// And be published/updated since the user max history.
						query.Bool(
							query.Should(
								query.Since("published", user.GetMaxHistory()),
								query.Since("updated", user.GetMaxHistory()),
								query.ItemIDs(info.GetReadItems()...),
							),
							// Must not match any unread items for the feed
							query.MustNot(
								query.ItemIDs(info.GetUnreadItems()...),
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
						query.Term("feed_id", feedID),
						// And should be between the user max history and last read time.
						query.Bool(
							query.Should(
								query.Between("published", user.GetMaxHistory(), info.GetMarkedRead()),
								query.Between("updated", user.GetMaxHistory(), info.GetMarkedRead()),
								query.ItemIDs(info.GetReadItems()...),
							),
							// Must not match any unread items for the feed
							query.MustNot(
								query.ItemIDs(info.GetUnreadItems()...),
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

func allFeedItemsQuery(user *models.User, subscriptions models.Subscriptions) query.Option {
	clauses := make([]query.Option, 0, len(subscriptions))

	for feedID := range subscriptions.ByFeed() {
		clauses = append(clauses,
			query.Bool(
				query.Filter(
					// Must match this feed.
					query.Term("feed_id", feedID),
					// And be published/updated since the user max history.
					query.Bool(
						query.Should(
							query.Since("published", user.GetMaxHistory()),
							query.Since("updated", user.GetMaxHistory()),
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
