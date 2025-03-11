// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/query"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var ErrUserActionFailed = errors.New("user action failed")
var ErrUserAlreadySubscribed = errors.New("user already subscribed")

// UserActionMarkItemsRead will mark the given items with the given state for the user.
func UserActionMarkItems(ctx context.Context, esapi *typedapi.API, mark api.Mark, ids ...models.ItemID) error {
	if mark != api.MarkUnread && mark != api.MarkRead {
		return fmt.Errorf("unsupported mark")
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}

	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrUpdateFailed, ErrNoIndexInCtx)
	}

	resp, err := NewSearchRequest(esapi,
		WithSearchIndex(index),
		WithFields("feed_id"),
		WithSearchQueryOptions(
			// Must have the  itemID
			query.ItemIDs(ids...),
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(len(ids)),
	).Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	feedIDs, warnings := ExtractFieldFromHits[models.FeedID]("feed_id", resp.Hits.Hits)
	if warnings != nil {
		logging.FromContext(ctx).Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", warnings))
	}

	// Mark all items with the given state.
	for _, itemID := range ids {
		feedID, found := feedIDs[itemID]
		if !found {
			continue
		}

		if err := user.MarkItem(feedID, itemID, models.State(mark)); err != nil && !errors.Is(err, models.ErrUserAlreadyReadItem) {
			logging.FromContext(ctx).Warn("Could not mark item read", slog.Any("error", err))
		}
	}

	// Update the user object.
	return UpdateUser(ctx, esapi, user.ID, map[string]any{
		"feed_item_states": user.FeedItemStates,
		"updated_at":       time.Now().UTC(),
	})
}

// UserActionMarkFeedsRead will mark the given feeds with the given state for
// the user.
func UserActionMarkFeeds(ctx context.Context, esapi *typedapi.API, mark api.Mark, feedIDs ...models.FeedID) error {
	if mark != api.MarkRead && mark != api.MarkUnread {
		return errors.Join(ErrUserActionFailed, errors.New("unsupported mark action"))
	}

	var timestamp time.Time

	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}

	// Based on the requested state change, calculate the marked read timestamp
	// for the feed.
	// For read state, this will be the current time.
	// For unread state, this will be the max history of the user.
	switch mark {
	case api.MarkRead:
		timestamp = time.Now().UTC()
	case api.MarkUnread:
		timestamp = user.GetMaxHistory()
	}

	// Mark all items read in the user object. Any items already marked read are ignored.
	for _, feed := range feedIDs {
		if err := user.MarkFeedRead(feed, timestamp); err != nil && !errors.Is(err, models.ErrUserAlreadyReadItem) {
			logging.FromContext(ctx).Warn("Could not mark item read", slog.Any("error", err))
		}
	}

	// Update the user object.
	return UpdateUser(ctx, esapi, user.ID, map[string]any{
		"feed_item_states": user.FeedItemStates,
		"subscriptions":    user.Subscriptions,
		"updated_at":       time.Now().UTC(),
	})
}

// GetItem retrieves the specified item with the given id and from the given
// feed. It checks for a subscription and will return false (without an error)
// if the current user is not subscribed.
func UserActionGetItem(ctx context.Context, api *typedapi.API, feedID models.FeedID, itemID models.ItemID) (models.APIItem, bool, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return models.APIItem{}, false, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.APIItem{}, false, ErrNoUserCtx
	}

	if !user.IsSubscribed(feedID) {
		return models.APIItem{}, false, ErrNoUserCtx
	}

	req := NewSearchRequest(api,
		WithSearchIndex(index),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			query.Bool(
				query.Filter(
					// Must have the feedID and itemID
					query.FeedIDs(feedID),
					query.ItemIDs(itemID),
					// Must be published or updated after the user max history.
					query.Bool(
						query.Should(
							query.Since("publishedParsed", user.GetFeedLastRead(feedID)),
							query.Since("updatedParsed", user.GetFeedLastRead(feedID)),
						),
					),
				),
			),
		),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return models.APIItem{}, false, errors.Join(ErrSearchFailed, err)
	}

	item, err := ExtractSource[models.APIItem](res.Hits.Hits[0].Source_)
	if err != nil {
		return models.APIItem{}, false, errors.Join(ErrSearchFailed, err)
	}

	return item, true, nil
}

// UserGetItems will search Elasticsearch for unread items (with
// given filters applied) for the given user, and, returns the items as well as
// pagination details for paging through the results.
func UserActionGetItems(ctx context.Context, api *typedapi.API, filters api.Filters) ([]*models.APIItem, api.Pagination, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", ErrGetUserFailed
	}

	// Work out what query to use based on the state filter.
	query := generateItemsQueryClause(user, filters)

	// Search through items matching any given feeds filters, excluding any read
	// items.
	resp, err := ItemsSearch(ctx, api, query, filters)
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}
	// Extract items and pagination values.
	items, lastSortValue, warnings := ExtractSourceFromHits[*models.APIItem](resp.Hits.Hits)
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
	for _, item := range items {
		// Add the state for the item from the user object, to the item object.
		if itemState := user.GetItemState(item); itemState != nil {
			item.SetUserItemState(itemState.State)
		}
	}

	return items, pagination, nil
}

// UserActionGetFeeds will search Elasticsearch for subscribed feeds (with
// given filters applied) for the given user, and, returns the feeds.
func UserActionGetFeeds(ctx context.Context, esapi *typedapi.API, filters api.Filters) ([]*models.APIFeed, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	filters.SetFeeds(filterSubscribedFeeds(user, filters)...)

	// Get the feed details for the subscribed feeds.
	feeds, err := FeedsSearch(ctx, esapi, filters)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	filters.Categories = nil
	// Get the unread counts for the feeds.
	countResults, err := ItemsAggregation(ctx, esapi, unreadFeedItemsQuery(user, filters), NewTermsAggregation("UnreadCounts", "feed_id"))
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	var categoryCounts TermsAggregationResults

	categoryCounts.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](countResults, "UnreadCounts")
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	unreadCounts := make(map[string]int)
	for _, feedID := range filters.GetFeeds() {
		unreadCounts[feedID] = categoryCounts.GetCount(feedID)
	}

	validFeeds := make([]*models.APIFeed, 0, len(feeds))

	for _, feed := range feeds {
		// Add user data to feed.
		addUserDataToFeed(user, feed, int(unreadCounts[feed.ID]))
		// If filtering by unread, ignore feeds with no unread count.
		if filters.ViewUnread() && feed.GetUserUnreadCount() == 0 {
			continue
		}
		// If filtering by read, ignore feeds with  unread count.
		if filters.ViewRead() && feed.GetUserUnreadCount() > 0 {
			slog.Info("not showing feed", slog.String("feed", feed.GetTitle()))
			continue
		}
		// Append to valid feeds list.
		validFeeds = append(validFeeds, feed)
	}
	// If the sort_by filters is unread count, sort the list of feeds by user
	// unread count. We can't do this in Elasticsearch as the unread count comes
	// from an aggregation and is not a field on the feed documents.
	if filters.GetSort().SortBy == api.UnreadCount {
		slices.SortFunc(validFeeds, cmpFeedUnreadCount)
		if filters.GetSort().SortOrder == api.SortDesc {
			slices.Reverse(validFeeds)
		}
	}

	return validFeeds, nil
}

func (c *Client) UserActionGetFeed(ctx context.Context, esapi *typedapi.API, feedID models.FeedID) (*models.APIFeed, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	if !user.IsSubscribed(feedID) {
		return nil, errors.Join(ErrUserActionFailed, ErrNoHits)
	}

	// Get the feed.
	feed, err := c.GetFeedByID(ctx, feedID)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	query := query.Bool(
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", feedID),
			// Must not match any read item IDs.
			query.ItemIDs(user.GetItemIDsWithState(models.Read, feedID)...),
			// And should be newer than last read or explicitly marked unread.
			query.Bool(
				query.Should(
					query.Since("publishedParsed", user.GetFeedLastRead(feedID)),
					query.Since("updatedParsed", user.GetFeedLastRead(feedID)),
					query.ItemIDs(user.GetItemIDsWithState(models.Unread, feedID)...),
				),
			),
		),
	)

	resp, err := ItemsCount(ctx, esapi, query)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	// Add user data to feed.
	addUserDataToFeed(user, feed, int(resp.Count))

	return feed, nil
}

func UserActionGetFeedCategories(ctx context.Context, esapi *typedapi.API) ([]api.CategoryCount, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	counts := make(map[models.Category]int)

	// Tally the count of categories across the user's subscriptions.
	for _, subscription := range user.Subscriptions {
		for _, category := range subscription.Categories {
			counts[category]++
		}
	}

	categoryCounts := make([]api.CategoryCount, 0, len(counts))

	// Reformat counts into CategoryCount objects.
	for category, count := range counts {
		categoryCounts = append(categoryCounts, api.CategoryCount{Category: category, Count: count})
	}

	return categoryCounts, nil
}

func UserActionGetItemCategories(ctx context.Context, esapi *typedapi.API, filters api.Filters) ([]api.CategoryCount, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	// Unset the category filter.
	filters.Categories = nil

	resp, err := ItemsAggregation(ctx, esapi, generateItemsQueryClause(user, filters), NewTermsAggregation("categories", "categories.raw"))
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	var results TermsAggregationResults

	results.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](resp, "categories")
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	categories := make([]api.CategoryCount, 0, results.BucketCount())

	for _, category := range results.BucketNames() {
		categories = append(categories, api.CategoryCount{Category: category, Count: results.GetCount(category)})
	}

	return categories, nil
}

// AddSubscription adds a new subscription to the user object.
func UserActionAddSubscription(ctx context.Context, esapi *typedapi.API, user *models.User, feedID models.FeedID, details *api.SubscriptionRequest) error {
	if user.IsSubscribed(feedID) {
		return ErrUserAlreadySubscribed
	}

	if user.Subscriptions == nil {
		user.Subscriptions = make(map[string]models.SubscriptionState)
	}

	user.Subscriptions[feedID] = api.NewSubscriptionState(details)

	partialUpdate := map[string]any{
		"subscriptions": user.Subscriptions,
	}

	if err := UpdateUser(ctx, esapi, user.GetID(), partialUpdate); err != nil {
		return errors.Join(models.ErrUpdateUser, err)
	}

	return nil
}

// generateItemsQueryClause selects the appropriate query clause for retrieving
// items using the given filters.
func generateItemsQueryClause(user *models.User, filters api.Filters) query.Option {
	// Work out what query to use based on the state filter.
	switch {
	case filters.ViewRead():
		return readFeedItemsQuery(user, filters)
	case filters.ViewUnread():
		return unreadFeedItemsQuery(user, filters)
	case filters.ViewAll():
		return allFeedItemsQuery(user, filters)
	default:
		return unreadFeedItemsQuery(user, filters)
	}
}

// unreadFeedItemsQuery generates a query for matching unread items using the given filters.
func unreadFeedItemsQuery(user *models.User, filters api.Filters) query.Option {
	clauses := make([]query.Option, 0, len(filters.GetFeeds()))
	for _, id := range filters.GetFeeds() {
		clauses = append(clauses,
			query.Bool(
				query.Filter(
					// Must match this feed.
					query.Term("feed_id", id),
					// And should be newer than last read or explicitly marked unread.
					query.Bool(
						query.Should(
							query.Since("publishedParsed", user.GetFeedLastRead(id)),
							query.Since("updatedParsed", user.GetFeedLastRead(id)),
							query.ItemIDs(user.GetItemIDsWithState(models.Unread, id)...),
						),
					),
				),
			),
		)
	}

	return query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(filters.GetFeeds()...),
			query.Categories(filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
		query.MustNot(
			// Must not match any read item IDs.
			query.ItemIDs(user.GetItemIDsWithState(models.Read, filters.GetFeeds()...)...),
		),
	)
}

// readFeedItemsQuery generates a query for matching read items using the given filters.
func readFeedItemsQuery(user *models.User, filters api.Filters) query.Option {
	readFeedIDs := make([]models.FeedID, 0, len(filters.GetFeeds()))
	clauses := make([]query.Option, 0, len(filters.GetFeeds()))

	for _, id := range filters.GetFeeds() {
		// Get any unread items for the feed.
		readFeedIDs = append(readFeedIDs, id)

		// Ignore feed if user has never marked it as read.
		if user.GetFeedLastRead(id) == user.GetMaxHistory() {
			clauses = append(clauses,
				query.Bool(
					query.Filter(
						// Must match this feed.
						query.Term("feed_id", id),
						// And be published/updated since the user max history.
						query.Bool(
							query.Should(
								query.Since("publishedParsed", user.GetMaxHistory()),
								query.Since("updatedParsed", user.GetMaxHistory()),
								query.ItemIDs(user.GetItemIDsWithState(models.Read, id)...),
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
						query.Term("feed_id", id),
						// And should be between the user max history and last read time.
						query.Bool(
							query.Should(
								query.Between("publishedParsed", user.GetMaxHistory(), user.GetFeedLastRead(id)),
								query.Between("updatedParsed", user.GetMaxHistory(), user.GetFeedLastRead(id)),
								query.ItemIDs(user.GetItemIDsWithState(models.Read, id)...),
							),
						),
					),
				),
			)
		}
	}

	return query.Bool(
		query.Filter(
			// Must match any of the given feed IDs
			query.FeedIDs(readFeedIDs...),
			query.Categories(filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
		query.MustNot(
			// Must not match an unread item.
			query.ItemIDs(user.GetItemIDsWithState(models.Unread, readFeedIDs...)...),
		),
	)
}

func allFeedItemsQuery(user *models.User, filters api.Filters) query.Option {
	clauses := make([]query.Option, 0, len(filters.GetFeeds()))

	for _, id := range filters.GetFeeds() {
		clauses = append(clauses,
			query.Bool(
				query.Filter(
					// Must match this feed.
					query.Term("feed_id", id),
					// And be published/updated since the user max history.
					query.Bool(
						query.Should(
							query.Since("publishedParsed", user.GetMaxHistory()),
							query.Since("updatedParsed", user.GetMaxHistory()),
						),
					),
				),
			),
		)
	}

	return query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.FeedIDs(filters.GetFeeds()...),
			query.Categories(filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(clauses...),
			),
		),
	)
}

// FilterSubscribedFeeds returns the user's subscribed feeds filtered by the
// given feed IDs.
func filterSubscribedFeeds(user *models.User, filters api.Filters) []models.FeedID {
	// If there are no relevant filters, return all subscribed Feed IDs.
	if len(filters.GetFeeds()) == 0 && len(filters.GetCategories()) == 0 {
		return user.GetSubscribedFeedIDs()
	}

	var filtered []models.FeedID

	switch {
	// Case 1: FeedID filters specified, no Category filters specified.
	case len(filters.GetFeeds()) > 0 && len(filters.GetCategories()) == 0:
		for _, id := range filters.GetFeeds() {
			if user.IsSubscribed(id) {
				filtered = append(filtered, id)
			}
		}

		return filtered
	// Case 2: No FeedID filters specified, Category filters specified.
	case len(filters.GetFeeds()) == 0 && len(filters.GetCategories()) > 0:
		for id, details := range user.Subscriptions {
			for _, category := range details.Categories {
				if slices.Contains(filters.GetCategories(), category) {
					filtered = append(filtered, id)
				}
			}
		}

		return filtered
	// Case 3: Both FeedID and Category filters specified
	default:
		for _, id := range filters.GetFeeds() {
			if user.IsSubscribed(id) {
				for _, category := range filters.GetCategories() {
					if user.SubscriptionHasCategory(id, category) {
						filtered = append(filtered, id)
					}
				}
			}
		}

		return filtered
	}
}

// addUserDataToFeed enriches an APIFeed with user specific data. This includes
// the user's unread count, nickname and custom categories for the Feed.
func addUserDataToFeed(user *models.User, feed *models.APIFeed, unread int) {
	// Add user unread count to feed.
	feed.SetUserUnreadCount(unread)
	// Add user defined name to the feed.
	if name := user.GetSubscriptionName(feed.GetID()); name != "" {
		feed.SetUserName(name)
	}
	// Add user defined categories to the feed.
	if categories := user.GetSubscriptionCategories(feed.GetID()); len(categories) > 0 {
		feed.SetUserCategories(categories)
	}
}

// cmpFeedUnreadCount is a helper function for sorting Feeds by unread count, in
// ascending order. If descending order is required, slices.Reverse can be
// called after sorting the slice with this function.
func cmpFeedUnreadCount(a, b *models.APIFeed) int {
	return cmp.Compare(a.GetUserUnreadCount(), b.GetUserUnreadCount())
}
