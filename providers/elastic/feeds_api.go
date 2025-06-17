// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (*bulk.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	bulkOps, respCh := bulk.NewRequest(ctx, e)

	go func() {
		defer close(bulkOps)

		for _, item := range items {
			slogctx.FromCtx(ctx).Debug("Adding item",
				slog.String("name", item.GetTitle()),
				slog.String("item_id", item.GetID()),
				slog.String("feed_id", item.GetFeedID()),
			)

			bulkOps <- bulk.NewOperation(&item,
				bulk.SetDocID(item.GetID()),
				bulk.ToIndex(index),
			)
		}
	}()

	resp := <-respCh

	return &resp, nil
}

// AddFeeds will bulk index the given feeds.
func (e *API) AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	bulkOps, respCh := bulk.NewRequest(ctx, e)

	go func() {
		defer close(bulkOps)

		for _, feed := range feeds {
			slogctx.FromCtx(ctx).Debug("Adding feed",
				slog.String("name", feed.GetTitle()),
				slog.String("feed_id", feed.GetID()),
			)

			bulkOps <- bulk.NewOperation(&feed,
				bulk.SetDocID(feed.GetID()),
				bulk.ToIndex(index),
			)
		}
	}()

	resp := <-respCh

	return &resp, nil
}

// AddSubscriptions will bulk index the given subscriptions.
func (e *API) AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) (*bulk.Response, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	bulkOps, respCh := bulk.NewRequest(ctx, e)

	go func() {
		defer close(bulkOps)

		for _, subscription := range subscriptions {
			slogctx.FromCtx(ctx).Debug("Adding feed",
				slog.String("name", subscription.GetTitle()),
				slog.String("feed_id", subscription.GetID()),
			)

			bulkOps <- bulk.NewOperation(&subscription,
				bulk.SetDocID(subscription.GetID()),
				bulk.ToIndex(index),
			)
		}
	}()

	resp := <-respCh

	return &resp, nil
}

// GetFeeds retrieves the feeds with the given IDs.
func (e *API) GetSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionCustomisations, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewMGetRequest(e.GetAPI(),
		GetFromIndex(index),
		GetIDs(ids...)).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetFailed, err)
	}
	subscriptions, warnings := ExtractSourceFromDocs[*models.SubscriptionCustomisation](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", warnings))
	}
	return subscriptions, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
//
// count specifies the number of results. If not specified, up to 10 results will be returned.
//
// sort specifies how to sort the resuls. If not specified, doc value sorting is used.
//
// pagination specifies the sort after values to use for getting a specific window of the total results. When set, the
// count parameter can be thought of as specifying how many new results are retrieved.
func (a *API) SearchSubscriptions(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.SubscriptionCustomisations, models.Pagination, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = decodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}
	var sortOptions []types.SortCombinationsVariant
	if sort != nil {
		sortOptions = setFeedSort(*sort)
	} else {
		sortOptions = append(sortOptions, SortByDocID("subscription_id"))
	}

	resp, err := NewSearchRequest(a.API,
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(count),
		WithSortOptions(sortOptions...),
	).Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}

	var warnings error
	var subscriptions []*models.SubscriptionCustomisation

	subscriptions, sortValues, warnings = ExtractSourceFromHits[*models.SubscriptionCustomisation](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some feeds could not be extracted from results.",
			slog.Any("warnings", warnings))
	}

	if pagination != nil {
		pagination, err := encodePagination(sortValues)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
		return subscriptions, pagination, nil
	}

	return subscriptions, "", nil
}

// GetFeeds retrieves the feeds with the given IDs.
func (e *API) GetFeeds(ctx context.Context, feedIDs ...models.FeedID) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	resp, err := NewMGetRequest(e.GetAPI(),
		GetFromIndex(index),
		GetIDs(feedIDs...)).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetFailed, err)
	}
	feeds, warnings := ExtractSourceFromDocs[*models.Feed](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some feeds could not be extracted from docs.",
			slog.Any("warnings", warnings))
	}
	return feeds, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
//
// count specifies the number of results. If not specified, up to 10 results will be returned.
//
// sort specifies how to sort the resuls. If not specified, doc value sorting is used.
//
// pagination specifies the sort after values to use for getting a specific window of the total results. When set, the
// count parameter can be thought of as specifying how many new results are retrieved.
func (a *API) SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = decodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}
	var sortOptions []types.SortCombinationsVariant
	if sort != nil {
		sortOptions = setFeedSort(*sort)
	} else {
		sortOptions = append(sortOptions, SortByDocID("feed_id"))
	}

	resp, err := NewSearchRequest(a.API,
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(count),
		WithSortOptions(sortOptions...),
	).Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}

	var warnings error
	var feeds models.Feeds

	feeds, sortValues, warnings = ExtractSourceFromHits[*models.Feed](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some feeds could not be extracted from results.",
			slog.Any("warnings", warnings))
	}

	if pagination != nil {
		*pagination, err = encodePagination(sortValues)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
//
// count specifies the number of results. If not specified, up to 10 results will be returned.
//
// sort specifies how to sort the resuls. If not specified, doc value sorting is used.
//
// pagination specifies the sort after values to use for getting a specific window of the total results. When set, the
// count parameter can be thought of as specifying how many new results are retrieved.
func (e *API) SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = decodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}
	var sortOptions []types.SortCombinationsVariant
	if sort != nil {
		sortOptions = setItemSort(*sort)
	} else {
		sortOptions = append(sortOptions, SortByDocID("item_id"))
	}

	resp, err := NewSearchRequest(e.GetAPI(),
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(count),
		WithSortOptions(sortOptions...),
	).Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}

	var warnings error
	var items models.Items

	items, sortValues, warnings = ExtractSourceFromHits[*models.Item](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some items could not be extracted from results.",
			slog.Any("warnings", warnings))
	}

	newPagination, err := encodePagination(sortValues)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}

	return items, newPagination, nil
}

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchSize(0),
		WithAggregations(aggregations...),
		WithSortOptions(setItemSort(models.NewFilters().Sort())...),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	return resp, nil
}

func (a *API) MultiSearch(ctx context.Context, feedsSearch, itemsSearch *query.MSearchOptions) (models.Feeds, models.Items, error) {
	subscriptionsIndex := FeedsIndexFromCtx(ctx)
	if subscriptionsIndex == "" {
		return nil, nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	itemsIndex := ItemsIndexFromCtx(ctx)
	if itemsIndex == "" {
		return nil, nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	req := NewMSearchRequest(a.GetAPI(),
		WithSearch[*msearch.Msearch](subscriptionsIndex, feedsSearch),
		WithSearch[*msearch.Msearch](itemsIndex, itemsSearch),
		WithRequestID[*msearch.Msearch, SearchRequestCommon[*msearch.Msearch]](middleware.GetReqID(ctx)),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	results := make(map[string]*types.MultiSearchItem)

	for idx, r := range []string{"subscriptions", "items"} {
		switch res := resp.Responses[idx].(type) {
		case *types.MultiSearchItem:
			if res.Hits.Total.Value == 0 {
				continue
			}
			results[r] = res
		case types.ErrorResponseBase:
		default:
		}
	}

	var (
		feeds    models.Feeds
		items    models.Items
		warnings error
	)
	if results["subscriptions"] != nil {
		feeds, _, warnings = ExtractSourceFromHits[*models.Feed](results["subscriptions"].Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problem extracting hits.", slog.Any("err", err))
		}
	}
	if results["items"] != nil {
		items, _, warnings = ExtractSourceFromHits[*models.Item](results["items"].Hits.Hits)
		if warnings != nil {
			slogctx.FromCtx(ctx).Warn("Problem extracting hits.", slog.Any("err", err))
		}
	}

	return feeds, items, nil
}

func (e *API) UpdateSubscriptionCustomisation(ctx context.Context, id models.SubscriptionID, partialUpdate map[string]any) error {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return fmt.Errorf("could not update subscription: %w", ErrGetUserFailed)
	}

	// Updated the `updated_at` timestamp.
	// partialUpdate["updated_at"] = time.Now().UTC()

	// Update the user in the store with the new list of read items.
	resp, err := NewDocUpdateRequest(e.GetAPI(), index, id,
		WithPartialDocUpdate(partialUpdate),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("could not update subscription: %w", err)
	}

	slog.Debug("Updated subscription.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}

// MarkFeedUpdated updates the timestamp indicating when the feed was last updated (i.e., new items found and indexed).
func (e *API) MarkFeedUpdated(ctx context.Context, feedID models.FeedID) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	partialUpdate := make(map[string]any)
	// Updated the `updated` timestamp.
	partialUpdate["updated"] = time.Now().UTC()
	// Update the user in the store with the new list of read items.
	resp, err := NewDocUpdateRequest(e.GetAPI(), index, feedID,
		WithPartialDocUpdate(partialUpdate),
	).Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}
	slogctx.FromCtx(ctx).Debug("Updated feed.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))
	return nil
}

// setFeedSort will define how Feeds are sorted based on the given sort options (from filters).
func setFeedSort(sort models.Sort) []types.SortCombinationsVariant {
	// Adjust based on given sort options.
	if sort.SortBy == models.SortByLastUpdated {
		switch sort.SortOrder {
		case models.SortOrderAsc:
			return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("feed_id", models.SortOrderDesc)}
		case models.SortOrderDesc:
			return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("feed_id", models.SortOrderDesc)}
		}
	}
	return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("feed_id", models.SortOrderDesc)}
}

// setItemSort will define how Items are sorted based on the given sort options (from filters).
func setItemSort(sort models.Sort) []types.SortCombinationsVariant {
	// Adjust based on given sort options.
	if sort.SortBy == models.SortByLastUpdated {
		switch sort.SortOrder {
		case models.SortOrderAsc:
			return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("item_id", models.SortOrderDesc)}
		case models.SortOrderDesc:
			return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("item_id", models.SortOrderDesc)}
		}
	}
	return []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("item_id", models.SortOrderDesc)}
}
