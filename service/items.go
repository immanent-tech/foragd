// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/maypok86/otter/v2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

var itemsCache = otter.Must(&otter.Options[models.ItemID, models.Item]{
	MaximumSize: 10_000,
})

// GetItems retrieves the Items matching the given ItemIDs.
func GetItems(ctx context.Context, ids ...models.ItemID) (models.Items, error) {
	var (
		items    models.Items
		err      error
		unCached []models.ItemID
	)

	// Fetch items from cache.
	for id := range slices.Values(ids) {
		if item, found := itemsCache.GetIfPresent(id); found {
			items = append(items, &item)
		} else {
			unCached = append(unCached, id)
		}
	}
	// If there are items missing from the cache, fetch and cache them.
	if len(unCached) > 0 {
		items, err = elastic.GetDocs[models.ItemID, *models.Item](ctx, schema.ItemsIndexRO(), ids...)
		if err != nil {
			return nil, fmt.Errorf("get items: %w", err)
		}
		for item := range slices.Values(items) {
			items = append(items, item)
			itemsCache.Set(item.GetID(), *item)
		}
	}
	return items, nil
}

// CountItems returns a count of items that match the given query.
func CountItems(ctx context.Context, query query.Option) (int64, error) {
	count, err := elastic.Count(ctx, schema.ItemsIndexRO(), query)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}

	return count, nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func SearchItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Items, models.Pagination, error) {
	searchAfter, err := elastic.DecodePagination(pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx, schema.ItemsIndexRO(), query,
		elastic.WithSort(models.NewItemSortOptions(sort)...),
		elastic.WithSearchAfter(searchAfter...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := elastic.EncodePagination[models.Pagination](resp.Pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	return resp.Results, newPagination, nil
}

// AddItems wraps an elastic bulk update to index items.
func AddItems(ctx context.Context, items ...*models.Item) error {
	if _, err := elastic.BulkUpdate(ctx, schema.ItemsIndexRW(), items...); err != nil {
		return fmt.Errorf("bulk add items: %w", err)
	}
	return nil
}

// BuildItemQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildItemQueries(
	user *models.User,
	view models.View,
	subscriptions models.Subscriptions,
) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		// Ignore subscriptions that aren't based on a feed object.
		if subscription.GetFeedID() == "" {
			continue
		}

		switch view {
		case models.ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case models.ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case models.ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *models.User, source models.ItemSource) query.Option {
	// if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
	// 	return nil
	// }
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Between("updated", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(_ *models.User, source models.ItemSource) query.Option {
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", source.GetMarkedReadAt()),
					query.Since("updated", source.GetMarkedReadAt()),
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// Must not match any read items for the feed
		query.MustNot(
			query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *models.User, source models.ItemSource) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		models.ArticleFiltersQueryClause(source),
	)
}
