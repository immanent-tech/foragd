// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var ErrUserActionFailed = errors.New("user action failed")

// UserActionMarkItemsRead will mark the given items with the given state for the user.
func (c *Client) UserActionMarkItems(ctx context.Context, mark models.Mark, ids []models.ItemID) error {
	if mark != models.MarkUnread && mark != models.MarkRead {
		return fmt.Errorf("unsupported mark.")
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
		WithIndex[*search.Search](index),
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
func (c *Client) UserActionMarkFeeds(ctx context.Context, mark models.Mark, feedIDs []models.FeedID) error {
	if mark != models.MarkRead && mark != models.MarkUnread {
		return fmt.Errorf("unsupported mark.")
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
func (c *Client) UserActionGetItem(ctx context.Context, feedID, itemID string) (models.APIItem, bool, error) {
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
		WithIndex[*search.Search](index),
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

	var query Option[*types.Query]
	// Work out what query to use based on the state filter.
	switch filters.GetView() {
	case models.Read:
		query = readFeedItemsQuery(user, filters.GetFeedIDs()...)
	case models.Unread:
		fallthrough
	default:
		query = unreadFeedItemsQuery(user, filters.GetFeedIDs()...)
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
		WithIndex[*search.Search](index),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			query,
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(filters.GetCount()),
		// WithSearchAfter(currentPagination),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrUserActionFailed, err)
	}

	items, warnings := ExtractSourceFromHits[models.APIItem](res.Hits.Hits)
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
func (c *Client) UserActionGetFeeds(ctx context.Context, filters models.APIFilters) (chan models.APIFeed, error) {
	outCh := make(chan models.APIFeed)

	user, found := models.UserFromCtx(ctx)
	if !found {
		return outCh, ErrGetUserFailed
	}

	// Get the FeedIDs for the user's subscriptions.
	subscribedFeedIDs := user.GetSubscribedFeedIDs()
	// Filter the list of requested feeds to only those which the user has a
	// subscription.
	if filters.FeedIDs != nil {
		subscribedFeedIDs = slices.Collect(
			func(yield func(string) bool) {
				for _, v := range subscribedFeedIDs {
					if slices.Contains(filters.GetFeedIDs(), v) {
						if !yield(v) {
							return // triggered in "break"
						}
					}
				}
			},
		)
	}
	// If there are no subscriptions, return an error indicating so.
	if len(subscribedFeedIDs) == 0 {
		return outCh, models.ErrNoSubscriptions
	}

	// Get the feed details for the subscribed feeds.
	feeds, err := c.GetFeedsByID(ctx, subscribedFeedIDs...)
	if err != nil {
		return outCh, errors.Join(ErrUserActionFailed, err)
	}

	// Get the unread counts for the feeds.
	unreadCounts, err := c.GetFeedItemCounts(ctx, user, filters.GetView(), subscribedFeedIDs)
	if err != nil {
		return outCh, errors.Join(ErrUserActionFailed, err)
	}
	// Add unread counts to feed objects.
	go func() {
		defer close(outCh)

		for _, feed := range feeds {
			feed.SetUserUnreadCount(int(unreadCounts[feed.ID]))
			outCh <- *feed
		}
	}()

	return outCh, nil
}
