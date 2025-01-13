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
func (c *Client) UserActionAddSubscriptions(ctx context.Context, subscriptions ...models.SubscriptionRequest) error {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}

	// Extract the URLs from the subscriptions.
	urls := make([]string, len(subscriptions))
	for idx, sub := range subscriptions {
		urls[idx] = sub.URL
	}
	// Get a list of existing existingFeeds by the subscription URLs.
	existingFeeds, err := c.userActionGetFeedsByURL(ctx, urls...)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	// Go through the requested subscriptions. Ignore any feeds the user has
	// already subscribed to. For the rest, add a new subscription.
	for _, subscription := range subscriptions {
		// Check for an existing feed for this subscription request.
		idx := slices.IndexFunc(existingFeeds, func(feed models.APIFeed) bool { return feed.URL == subscription.URL })
		if idx == -1 {
			// If there is no existing feed, create the feed in addition to the user
			// subscription.
			// create feed
			// create subscription
		} else {
			// Create a new subscription if the user is not already subscribed.
			if !user.IsSubscribed(existingFeeds[idx].ID) {
				// create subscription
			}
		}

		// user.Subscriptions[existingFeeds[subscriptions.URL]] = models.Subscription{
		// 	Categories: subscriptions.Categories,
		// 	Name:       subscriptions.Name,
		// }
	}

	// Update the user subscriptions.
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

// UserActionMarkItemsRead will mark the given items as read for the user.
func (c *Client) UserActionMarkItemsRead(ctx context.Context, items ...models.APIReadItem) error {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
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
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.APIItem{}, false, ErrNoUserCtx
	}

	if !user.IsSubscribed(feedID) {
		return models.APIItem{}, false, ErrNoUserCtx
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
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, nil, ErrGetUserFailed
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
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrGetUserFailed
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

	// If there are no subscriptions, return an error indicating so.
	if len(feedIDs) == 0 {
		return nil, models.ErrNoSubscriptions
	}

	// Get the feed details.
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

	// Get the user's read items for the list of feeds.
	itemIDs := user.GetReadItemIDs(feedIDs...)

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

func (c *Client) userActionGetFeedsByURL(ctx context.Context, urls ...string) ([]models.APIFeed, error) {
	searchSize := 1000
	pagination := make([]types.FieldValue, 0)
	feeds := make([]models.APIFeed, 0)

	// Loop until we've paginated through all results.
	for {
		resp, err := c.NewSearchRequest(
			WithIndexPattern[*search.Search](schema.FeedsSchemaPrefix+"-*"),
			WithFields("feed_id", "feedLink"),
			WithSearchQueryOptions(QueryByURLs("feedLink", urls...)),
			WithSearchSize(searchSize),
			WithSearchAfter(pagination),
		).Do(ctx)
		if err != nil {
			return nil, errors.Join(ErrSearchFailed, err)
		}
		// Stop if there are no hits or the number of hits is less than the
		// search size (i.e., last set of hits).
		if len(resp.Hits.Hits) == 0 || len(resp.Hits.Hits) < searchSize {
			break
		}
		// Loop through this set of results.
		feeds = append(feeds, extractSources[models.APIFeed](ctx, resp.Hits.Hits)...)
		// Update pagination value.
		pagination = resp.Hits.Hits[len(resp.Hits.Hits)-1].Sort
	}

	return feeds, nil
}
