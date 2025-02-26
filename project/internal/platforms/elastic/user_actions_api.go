// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var ErrUserActionFailed = errors.New("user action failed")

// UserActionMarkItemsRead will mark the given items with the given state for the user.
func (c *Client) UserActionMarkItems(ctx context.Context, mark models.Mark, ids ...models.ItemID) error {
	if mark != models.MarkUnread && mark != models.MarkRead {
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
func (c *Client) UserActionMarkFeeds(ctx context.Context, mark models.Mark, feedIDs ...models.FeedID) error {
	if mark != models.MarkRead && mark != models.MarkUnread {
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
	case models.MarkRead:
		timestamp = time.Now().UTC()
	case models.MarkUnread:
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
func (c *Client) UserActionGetItems(ctx context.Context, filters models.APIFilters) (chan models.APIItem, models.Pagination, error) {
	outCh := make(chan models.APIItem)

	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return outCh, "", errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	user, found := models.UserFromCtx(ctx)
	if !found {
		return outCh, "", ErrGetUserFailed
	}

	var query QueryOption
	// Work out what query to use based on the state filter.
	switch filters.View {
	case models.ViewRead:
		query = readFeedItemsQuery(user, filters)
	case models.ViewUnread:
		fallthrough
	default:
		query = unreadFeedItemsQuery(user, filters)
	}

	// rawPagination, err := filters.GetPagination()
	// if err != nil {
	// 	c.Logger.Debug("Could not get pagination value.",
	// 		slog.Any("error", err))
	// }

	// currentPagination, err := models.DecodePagination(rawPagination)
	// if err != nil {
	// 	c.Logger.Debug("Could not get pagination value.",
	// 		slog.Any("error", err))
	// }

	// Search through items matching any given feeds filters, excluding any read
	// items.
	req := c.NewSearchRequest(
		WithSearchIndex(index),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			query,
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(filters.Count),
		// WithSearchAfter(currentPagination),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}

	items, _, warnings := ExtractSourceFromHits[models.APIItem](res.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	var newPagination models.Pagination
	// Get the sort value(s) of the last hit.
	if len(res.Hits.Hits) > 0 {
		data, err := json.Marshal(res.Hits.Hits[len(res.Hits.Hits)-1].Sort)
		if err != nil {
			c.Logger.Warn("Cannot marshal sort value.", slog.Any("error", err))
		}

		newPagination, err = models.EncodePagination(data)
		if err != nil {
			c.Logger.Warn("Cannot marshal sort value.", slog.Any("error", err))
		}
	}

	go func() {
		defer close(outCh)

		for _, item := range items {
			// Add the state for the item from the user object, to the item object.
			if itemState := user.GetItemState(item.FeedID, item.ID); itemState != nil {
				item.SetUserItemState(itemState.State)
			}

			outCh <- item
		}
	}()

	return outCh, newPagination, nil
}

// UserActionGetFeeds will search Elasticsearch for subscribed feeds (with
// given filters applied) for the given user, and, returns the feeds.
func (c *Client) UserActionGetFeeds(ctx context.Context, filters models.APIFilters) ([]*models.APIFeed, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	filters.FeedIDs = user.FilterSubscribedFeeds(filters)

	// Get the feed details for the subscribed feeds.
	feeds, err := c.FeedsSearch(ctx, filters)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	// Reset filter categories to calculate counts over all categories.
	filters.Categories = nil
	// Get the unread counts for the feeds.
	countResults, err := c.ItemsAggregation(ctx, generateItemsQueryClause(user, filters), NewTermsAggregation("UnreadCounts", "feed_id"))
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	var categoryCounts TermsAggregationResults

	categoryCounts.StringTermsAggregate, err = ExtractAggregation[*types.StringTermsAggregate](countResults, "UnreadCounts")
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	unreadCounts := make(map[string]int64)
	for _, feedID := range filters.FeedIDs {
		unreadCounts[feedID] = categoryCounts.GetCount(feedID)
	}

	for _, feed := range feeds {
		// If filtering by unread, ignore feeds with no unread count.
		if filters.View == models.ViewUnread && int(unreadCounts[feed.ID]) == 0 {
			continue
		} else {
			// Add user unread count to feed.
			feed.SetUserUnreadCount(int(unreadCounts[feed.ID]))
		}
		// Add user defined name to the feed.
		if name := user.Subscriptions[feed.ID].Name; name != nil && *name != "" {
			feed.SetUserName(*name)
		}
		// Add user defined categories to the feed.
		if categories := user.Subscriptions[feed.ID].Categories; categories != nil {
			feed.SetUserCategories(categories)
		}
		// Pass the feed through the output channel.
	}

	return feeds, nil
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

func (c *Client) UserActionGetFeedCategories(ctx context.Context) ([]models.CategoryCount, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
	}

	// subscriptions := user.GetSubscribedFeedIDs()

	return user.GetCategoryCounts(), nil
}

func (c *Client) UserActionGetItemCategories(ctx context.Context, filters models.APIFilters) ([]models.CategoryCount, error) {
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

	categories := make([]models.CategoryCount, 0, results.BucketCount())

	for _, category := range results.BucketNames() {
		categories = append(categories, models.CategoryCount{Name: category, Count: results.GetCount(category)})
	}

	return categories, nil
}

// generateItemsQueryClause selects the appropriate query clause for retrieving
// items using the given filters.
func generateItemsQueryClause(user *models.User, filters models.APIFilters) QueryOption {
	// Work out what query to use based on the state filter.
	switch filters.View {
	case models.ViewRead:
		return readFeedItemsQuery(user, filters)
	case models.ViewUnread:
		fallthrough
	default:
		return unreadFeedItemsQuery(user, filters)
	}
}

// unreadFeedItemsQuery generates a query for matching unread items using the given filters.
func unreadFeedItemsQuery(user *models.User, filters models.APIFilters) QueryOption {
	clauses := make([]QueryOption, 0, len(filters.FeedIDs))
	for _, id := range filters.FeedIDs {
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
			QueryByFeedIDs(filters.FeedIDs...),
			QueryByCategory(filters.Categories...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
			),
		),
		BoolMustNot(
			// Must not match any read item IDs.
			QueryByItemIDs(user.GetItemIDsWithState(models.Read, filters.FeedIDs...)...),
		),
	)
}

// readFeedItemsQuery generates a query for matching read items using the given filters.
func readFeedItemsQuery(user *models.User, filters models.APIFilters) QueryOption {
	readFeedIDs := make([]models.FeedID, 0, len(filters.FeedIDs))
	clauses := make([]QueryOption, 0, len(filters.FeedIDs))

	for _, id := range filters.FeedIDs {
		// Ignore feed if user has never marked it as read.
		if user.GetFeedLastRead(id) == user.GetMaxHistory() {
			continue
		}

		// Get any unread items for the feed.
		readFeedIDs = append(readFeedIDs, id)
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
						),
					),
				),
			),
		)
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs
			QueryByFeedIDs(readFeedIDs...),
			QueryByCategory(filters.Categories...),
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
