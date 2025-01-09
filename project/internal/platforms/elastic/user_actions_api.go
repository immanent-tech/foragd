// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

// ErrUserActionFailed is a generic error indicating something went wrong with a
// user action request. Typically it should be joined with the actual error
// returned from any underlying methods.
var ErrUserActionFailed = errors.New("user action failed")

// UserActionAddSubscriptions will add subscriptions for the user.
func (c *Client) UserActionAddSubscriptions(ctx context.Context, subscriptions ...models.APISubscription) error {
	user, err := c.GetUser(ctx)
	if err != nil {
		return errors.Join(ErrGetUserFailed, err)
	}

	for _, subscription := range subscriptions {
		if _, found := user.Subscriptions[subscription.FeedID]; found {
			c.Logger.Warn("Already subscribed.")
			continue
		}

		user.Subscriptions[subscription.FeedID] = models.Subscription{
			Categories: subscription.Categories,
			Name:       subscription.Name,
		}
	}

	req := c.NewUpdateRequest(schema.UsersSchemaPrefix, user.ID,
		WithDocUpdate(user.Subscriptions),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	slog.Debug("Updated subscriptions.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}

// UserSubscriptionExists checks if the user has a valid subscription to the
// given feed.
func (c *Client) UserSubscriptionExists(ctx context.Context, feedID models.FeedID) (bool, error) {
	user, err := c.GetUser(ctx)
	if err != nil {
		return false, errors.Join(ErrGetUserFailed, err)
	}

	_, found := user.Subscriptions[feedID]

	return found, nil
}

// UserActionMarkItemsRead will mark the given items as read for the user.
func (c *Client) UserActionMarkItemsRead(ctx context.Context, items ...models.APIReadItem) error {
	user, err := c.GetUser(ctx)
	if err != nil {
		return errors.Join(ErrGetUserFailed, err)
	}

	for _, item := range items {
		// If there are read items for the feed and if the item is already
		// marked read, continue.
		if feed, found := user.ReadItems[item.FeedID]; found {
			if slices.ContainsFunc(feed, func(readitem models.ReadItem) bool {
				return readitem.ItemID == item.ItemID
			}) {
				continue
			}
		}
		// Add a new read item to the user record.
		user.ReadItems[item.FeedID] = append(user.ReadItems[item.FeedID], models.ReadItem{
			ItemID:    item.ItemID,
			Timestamp: time.Now(),
		})
	}

	req := c.NewUpdateRequest(schema.UsersSchemaPrefix, user.ID,
		WithDocUpdate(user.ReadItems),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	slog.Debug("Updated read items.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}

// GetItem retrieves the specified item with the given id and from the given
// feed. It checks for a subscription and will return false (without an error)
// if the current user is not subscribed.
func (c *Client) UserActionGetItem(ctx context.Context, feedID, itemID string) (models.APIItem, bool, error) {
	subscribed, err := c.UserSubscriptionExists(ctx, feedID)
	if err != nil {
		return models.APIItem{}, false, errors.Join(ErrUserActionFailed, err)
	}

	if !subscribed {
		return models.APIItem{}, false, nil
	}

	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryByFeedIDs(feedID),
			QueryByItemIDs(itemID)),
		WithSortOptions(SortTimestampDesc()),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return models.APIItem{}, false, errors.Join(ErrSearchFailed, err)
	}

	item, err := extractSource[models.APIItem](res.Hits.Hits[0].Source_)
	if err != nil {
		return models.APIItem{}, false, errors.Join(ErrSearchFailed, err)
	}

	return item, true, nil
}

// UserGetItems will search Elasticsearch for unread items (with
// given filters applied) for the given user, and, returns the items as well as
// pagination details for paging through the results.
func (c *Client) UserActionGetItems(ctx context.Context, filters models.APISearchFilters) ([]models.APIItem, []byte, error) {
	user, err := c.GetUser(ctx)
	if err != nil {
		return nil, nil, errors.Join(ErrGetUserFailed, err)
	}

	readItemsIDs := user.GetReadItemIDs(filters.FeedIDs...)

	// Search through items matching any given feeds filters, excluding any read
	// items.
	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(QueryByFeedIDs(filters.FeedIDs...)),
				BoolMustNot(QueryByItemIDs(readItemsIDs...)),
			),
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(filters.Count),
		WithSearchAfter(filters.Pagination),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, nil, errors.Join(ErrUserActionFailed, err)
	}

	items := extractSources[models.APIItem](ctx, res.Hits.Hits)

	var paginationData []byte
	// Get the sort value(s) of the last hit.
	if len(res.Hits.Hits) > 0 {
		paginationData, err = json.Marshal(res.Hits.Hits[len(res.Hits.Hits)-1].Sort)
		if err != nil {
			c.Logger.Warn("Cannot marshal sort value.", slog.Any("error", err))
		}
	}

	return items, paginationData, nil
}

// UserActionGetFeeds will search Elasticsearch for subscribed feeds (with
// given filters applied) for the given user, and, returns the feeds.
func (c *Client) UserActionGetFeeds(ctx context.Context, filters models.APISearchFilters) ([]models.APIFeed, error) {
	user, err := c.GetUser(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetUserFailed, err)
	}

	subscribedFeedIDs := user.GetSubscribedFeedIDs()

	// Filter the list of requested feeds to only those which the user has a
	// subscription.
	feedIDs := slices.Collect(
		func(yield func(string) bool) {
			for _, v := range subscribedFeedIDs {
				if slices.Contains(filters.FeedIDs, v) {
					if !yield(v) {
						return // triggered in "break"
					}
				}
			}
		},
	)

	itemIDs := user.GetReadItemIDs(feedIDs...)

	req := c.NewMGetRequest(
		WithIndexPattern[*mget.Mget](schema.FeedsSchemaPrefix),
		WithIDs(feedIDs...),
		// WithStoredFields(defaultFeedFields...),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []models.APIFeed

	c.userActionGetFeedUnreadCounts(ctx, feedIDs, itemIDs)

	for _, doc := range res.Docs {
		switch obj := doc.(type) {
		case types.MultiGetError:
			c.Logger.Warn("Problem getting document", slog.Any("error", obj))
		case *types.GetResult:
			feed, err := extractSource[models.APIFeed](obj.Source_)
			if err != nil {
				c.Logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
				continue
			}

			feeds = append(feeds, feed)
		}
	}

	return feeds, nil
}

// // getAllFeeds retrieves all feeds by executing a search request with a
// // match_all query.
// func (c *Client) getAllFeeds(ctx context.Context) ([]models.APIFeed, error) {
// 	resp, err := c.NewSearchRequest(
// 		WithIndexPattern[*search.Search](schema.FeedsSchemaPrefix+"-*"),
// 		WithFields(defaultFeedFields...),
// 		WithSearchQueryOptions(QueryMatchAll()),
// 	).Do(ctx)
// 	if err != nil {
// 		return nil, errors.Join(ErrSearchFailed, err)
// 	}

// 	feeds := extractSources[models.APIFeed](ctx, resp.Hits.Hits)

// 	return feeds, nil
// }

func (c *Client) userActionGetFeedUnreadCounts(ctx context.Context, feedIDs []models.FeedID, readItemIDs []models.ItemID) (map[string]int, error) {
	// Search through items matching any given feeds filters, excluding any read
	// items.
	req := c.NewSearchRequest(
		WithIndexPattern[*search.Search](schema.FeedItemsSchemaPrefix+"-*"),
		WithFields(defaultItemFields...),
		WithSearchQueryOptions(
			QueryBool(
				BoolFilter(QueryByFeedIDs(feedIDs...)),
				BoolMustNot(QueryByItemIDs(readItemIDs...)),
			),
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(0),
		WithAggregations(TermsAggregation("UnreadCounts", "feed_id")),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	unreadCounts := make(map[string]int)

	spew.Dump(resp.Aggregations["UnreadCounts"].(types.TermsAggregation))

	return unreadCounts, nil
}
