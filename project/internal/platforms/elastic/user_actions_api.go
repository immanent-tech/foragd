// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/models"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var ErrUserActionFailed = errors.New("user action failed")
var ErrUserAlreadySubscribed = errors.New("user already subscribed")

// UserActionMarkItemsRead will mark the given items with the given state for the user.
func (c *Client) UserActionMarkItems(ctx context.Context, mark api.Mark, ids ...models.ItemID) error {
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

	resp, err := c.NewSearchRequest(
		WithSearchIndex(index),
		WithFields("feed_id"),
		WithSearchQueryOptions(
			// Must have the  itemID
			QueryByItemIDs(ids...),
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(len(ids)),
	).Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	feedIDs, warnings := ExtractFieldFromHits[models.FeedID]("feed_id", resp.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", warnings))
	}

	// Mark all items with the given state.
	for _, itemID := range ids {
		feedID, found := feedIDs[itemID]
		if !found {
			continue
		}

		if err := user.MarkItem(feedID, itemID, models.State(mark)); err != nil && !errors.Is(err, models.ErrUserAlreadyReadItem) {
			c.Logger.Warn("Could not mark item read", slog.Any("error", err))
		}
	}

	// Update the user object.
	return c.UpdateUser(ctx, user.ID, map[string]any{
		"feed_item_states": user.FeedItemStates,
		"updated_at":       time.Now().UTC(),
	})
}

// UserActionMarkFeedsRead will mark the given feeds with the given state for
// the user.
func (c *Client) UserActionMarkFeeds(ctx context.Context, mark api.Mark, feedIDs ...models.FeedID) error {
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
			c.Logger.Warn("Could not mark item read", slog.Any("error", err))
		}
	}

	// Update the user object.
	return c.UpdateUser(ctx, user.ID, map[string]any{
		"feed_item_states": user.FeedItemStates,
		"subscriptions":    user.Subscriptions,
		"updated_at":       time.Now().UTC(),
	})
}

// GetItem retrieves the specified item with the given id and from the given
// feed. It checks for a subscription and will return false (without an error)
// if the current user is not subscribed.
func (c *Client) UserActionGetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (models.APIItem, bool, error) {
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

	req := c.NewSearchRequest(
		WithSearchIndex(index),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(
					// Must have the feedID and itemID
					QueryByFeedIDs(feedID),
					QueryByItemIDs(itemID),
					// Must be published or updated after the user max history.
					QueryBool(
						BoolShould(
							QuerySince("publishedParsed", user.GetFeedLastRead(feedID)),
							QuerySince("updatedParsed", user.GetFeedLastRead(feedID)),
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
func (c *Client) UserActionGetItems(ctx context.Context, filters api.Filters) ([]*models.APIItem, api.Pagination, error) {
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
	resp, err := c.ItemsSearch(ctx, query, filters.GetCount(), filters.GetPagination())
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}
	// Extract items and pagination values.
	items, lastSortValue, warnings := ExtractSourceFromHits[*models.APIItem](resp.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
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
func (c *Client) UserActionGetFeeds(ctx context.Context, filters api.Filters) ([]*models.APIFeed, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	filters.SetFeeds(filterSubscribedFeeds(user, filters)...)

	// Get the feed details for the subscribed feeds.
	feeds, err := c.FeedsSearch(ctx, filters)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	filters.Categories = nil
	// Get the unread counts for the feeds.
	countResults, err := c.ItemsAggregation(ctx, unreadFeedItemsQuery(user, filters), NewTermsAggregation("UnreadCounts", "feed_id"))
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
		// Add user unread count to feed.
		feed.SetUserUnreadCount(int(unreadCounts[feed.ID]))
		// If filtering by unread, ignore feeds with no unread count.
		if filters.View == api.ViewUnread && feed.GetUserUnreadCount() == 0 {
			continue
		}
		// If filtering by read, ignore feeds with  unread count.
		if filters.View == api.ViewRead && feed.GetUserUnreadCount() > 0 {
			slog.Info("not showing feed", slog.String("feed", feed.GetTitle()))
			continue
		}

		// Add user defined name to the feed.
		if name := user.Subscriptions[feed.ID].Name; name != nil && *name != "" {
			feed.SetUserName(*name)
		}
		// Add user defined categories to the feed.
		if categories := user.Subscriptions[feed.ID].Categories; categories != nil {
			feed.SetUserCategories(categories)
		}
		// Append to valid feeds list.
		validFeeds = append(validFeeds, feed)
	}

	return validFeeds, nil
}

func (c *Client) UserActionGetFeed(ctx context.Context, feedID models.FeedID) (*models.APIFeed, error) {
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

	query := QueryBool(
		BoolFilter(
			// Must match this feed.
			QueryByTerm("feed_id", feedID),
			// Must not match any read item IDs.
			QueryByItemIDs(user.GetItemIDsWithState(models.Read, feedID)...),
			// And should be newer than last read or explicitly marked unread.
			QueryBool(
				BoolShould(
					QuerySince("publishedParsed", user.GetFeedLastRead(feedID)),
					QuerySince("updatedParsed", user.GetFeedLastRead(feedID)),
					QueryByItemIDs(user.GetItemIDsWithState(models.Unread, feedID)...),
				),
			),
		),
	)

	resp, err := c.ItemsCount(ctx, query)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	// Add user unread count to feed.
	feed.SetUserUnreadCount(int(resp.Count))
	// Add user defined name to the feed.
	if name := user.Subscriptions[feed.ID].Name; name != nil && *name != "" {
		feed.SetUserName(*name)
	}
	// Add user defined categories to the feed.
	if categories := user.Subscriptions[feed.ID].Categories; categories != nil {
		feed.SetUserCategories(categories)
	}

	return feed, nil
}

func (c *Client) UserActionGetFeedCategories(ctx context.Context) ([]api.CategoryCount, error) {
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
		categoryCounts = append(categoryCounts, api.CategoryCount{Name: category, Count: count})
	}

	return categoryCounts, nil
}

func (c *Client) UserActionGetItemCategories(ctx context.Context, filters api.Filters) ([]api.CategoryCount, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	// Unset the category filter.
	filters.Categories = nil

	resp, err := c.ItemsAggregation(ctx, generateItemsQueryClause(user, filters), NewTermsAggregation("categories", "categories.raw"))
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
		categories = append(categories, api.CategoryCount{Name: category, Count: results.GetCount(category)})
	}

	return categories, nil
}

// AddSubscription adds a new subscription to the user object.
func UserActionAddSubscription(ctx context.Context, user *models.User, client *Client, feedID models.FeedID, details *api.SubscriptionRequest) error {
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

	if err := client.UpdateUser(ctx, user.ID, partialUpdate); err != nil {
		return errors.Join(models.ErrUpdateUser, err)
	}

	return nil
}

// generateItemsQueryClause selects the appropriate query clause for retrieving
// items using the given filters.
func generateItemsQueryClause(user *models.User, filters api.Filters) QueryOption {
	// Work out what query to use based on the state filter.
	switch filters.View {
	case api.ViewRead:
		return readFeedItemsQuery(user, filters)
	case api.ViewUnread:
		return unreadFeedItemsQuery(user, filters)
	case api.ViewAll:
		return allFeedItemsQuery(user, filters)
	default:
		return unreadFeedItemsQuery(user, filters)
	}
}

// unreadFeedItemsQuery generates a query for matching unread items using the given filters.
func unreadFeedItemsQuery(user *models.User, filters api.Filters) QueryOption {
	clauses := make([]QueryOption, 0, len(filters.GetFeeds()))
	for _, id := range filters.GetFeeds() {
		clauses = append(clauses,
			QueryBool(
				BoolFilter(
					// Must match this feed.
					QueryByTerm("feed_id", id),
					// And should be newer than last read or explicitly marked unread.
					QueryBool(
						BoolShould(
							QuerySince("publishedParsed", user.GetFeedLastRead(id)),
							QuerySince("updatedParsed", user.GetFeedLastRead(id)),
							QueryByItemIDs(user.GetItemIDsWithState(models.Unread, id)...),
						),
					),
				),
			),
		)
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs.
			QueryByFeedIDs(filters.GetFeeds()...),
			QueryByCategory(filters.GetCategories()...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
			),
		),
		BoolMustNot(
			// Must not match any read item IDs.
			QueryByItemIDs(user.GetItemIDsWithState(models.Read, filters.GetFeeds()...)...),
		),
	)
}

// readFeedItemsQuery generates a query for matching read items using the given filters.
func readFeedItemsQuery(user *models.User, filters api.Filters) QueryOption {
	readFeedIDs := make([]models.FeedID, 0, len(filters.GetFeeds()))
	clauses := make([]QueryOption, 0, len(filters.GetFeeds()))

	for _, id := range filters.GetFeeds() {
		// Get any unread items for the feed.
		readFeedIDs = append(readFeedIDs, id)

		// Ignore feed if user has never marked it as read.
		if user.GetFeedLastRead(id) == user.GetMaxHistory() {
			clauses = append(clauses,
				QueryBool(
					BoolFilter(
						// Must match this feed.
						QueryByTerm("feed_id", id),
						// And be published/updated since the user max history.
						QueryBool(
							BoolShould(
								QuerySince("publishedParsed", user.GetMaxHistory()),
								QuerySince("updatedParsed", user.GetMaxHistory()),
								QueryByItemIDs(user.GetItemIDsWithState(models.Read, id)...),
							),
						),
					),
				),
			)
		} else {
			clauses = append(clauses,
				QueryBool(
					BoolFilter(
						// Must match this feed.
						QueryByTerm("feed_id", id),
						// And should be between the user max history and last read time.
						QueryBool(
							BoolShould(
								QueryBetween("publishedParsed", user.GetMaxHistory(), user.GetFeedLastRead(id)),
								QueryBetween("updatedParsed", user.GetMaxHistory(), user.GetFeedLastRead(id)),
								QueryByItemIDs(user.GetItemIDsWithState(models.Read, id)...),
							),
						),
					),
				),
			)
		}
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs
			QueryByFeedIDs(readFeedIDs...),
			QueryByCategory(filters.GetCategories()...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
			),
		),
		BoolMustNot(
			// Must not match an unread item.
			QueryByItemIDs(user.GetItemIDsWithState(models.Unread, readFeedIDs...)...),
		),
	)
}

func allFeedItemsQuery(user *models.User, filters api.Filters) QueryOption {
	clauses := make([]QueryOption, 0, len(filters.GetFeeds()))

	for _, id := range filters.GetFeeds() {
		clauses = append(clauses,
			QueryBool(
				BoolFilter(
					// Must match this feed.
					QueryByTerm("feed_id", id),
					// And be published/updated since the user max history.
					QueryBool(
						BoolShould(
							QuerySince("publishedParsed", user.GetMaxHistory()),
							QuerySince("updatedParsed", user.GetMaxHistory()),
						),
					),
				),
			),
		)
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs.
			QueryByFeedIDs(filters.GetFeeds()...),
			QueryByCategory(filters.GetCategories()...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
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
