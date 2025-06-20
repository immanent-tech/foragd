// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/logging"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*typedapi.API
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *typedapi.API {
	return a.API
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortOptions []types.SortCombinationsVariant
	if sort != nil {
		if sort.SortBy == models.SortByLastUpdated {
			switch sort.SortOrder {
			case models.SortOrderAsc:
				sortOptions = []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("feed_id", models.SortOrderDesc)}
			case models.SortOrderDesc:
				sortOptions = []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("feed_id", models.SortOrderDesc)}
			}
		}
	} else {
		sortOptions = append(sortOptions, SortByDocID("_doc"))
	}

	feeds, nextResults, err := SearchDocs[*models.Feed](ctx, e.GetAPI(), index, query, count, sortOptions, pagination)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	return feeds, nextResults, nil
}

// AddFeeds will bulk index the given feeds.
func (e *API) AddFeeds(ctx context.Context, feeds ...*models.Feed) (map[models.FeedID]*bulk.OperationResponse, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, feeds...)
}

// GetFeeds retrieves the feeds with the given IDs.
func (e *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, e.GetAPI(), index, ids...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", err))
	}
	return feeds, nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortOptions []types.SortCombinationsVariant
	if sort != nil {
		if sort.SortBy == models.SortByLastUpdated {
			switch sort.SortOrder {
			case models.SortOrderAsc:
				sortOptions = []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderAsc), NewFieldSort("item_id", models.SortOrderDesc)}
			case models.SortOrderDesc:
				sortOptions = []types.SortCombinationsVariant{NewFieldSort("updated", models.SortOrderDesc), NewFieldSort("item_id", models.SortOrderDesc)}
			}
		}
	} else {
		sortOptions = append(sortOptions, SortByDocID("_doc"))
	}

	items, nextResults, err := SearchDocs[*models.Item](ctx, e.GetAPI(), index, query, count, sortOptions, pagination)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	return items, nextResults, nil
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, items...)
}

// SearchSubscriptionCustomisations will search the feeds index for feed matching the given query. Count, sort and
// pagination values are optional.
func (e *API) SearchSubscriptionCustomisations(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.SubscriptionCustomisations, models.Pagination, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}

	var sortOptions []types.SortCombinationsVariant
	sortOptions = append(sortOptions, SortByDocID("_doc"))

	customisations, nextResults, err := SearchDocs[*models.SubscriptionCustomisation](ctx, e.GetAPI(), index, query, count, sortOptions, pagination)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	return customisations, nextResults, nil
}

// AddSubscriptionCustomisations performs a bulk add operation to add the given subscription customisations.
func (e *API) AddSubscriptionCustomisations(ctx context.Context, customisations ...*models.SubscriptionCustomisation) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, customisations...)
}

// GetSubscriptionCustomisations retrieves the subscription customisations with the given IDs.
func (e *API) GetSubscriptionCustomisations(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionCustomisations, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	subscriptions, err := GetDocs[models.SubscriptionID, *models.SubscriptionCustomisation](ctx, e.GetAPI(), index, ids...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", err))
	}
	return subscriptions, nil
}

func (e *API) UpdateSubscriptionCustomisation(ctx context.Context, edits *models.SubscriptionEdit) error {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.ErrInvalidID
	}
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return ErrFetchCtx
	}

	found, err := NewDocExistsRequest(e.GetAPI(), index, edits.SubscriptionID).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	if !found {
		state := user.GetSubscriptionState(edits.SubscriptionID)
		customisation := &models.SubscriptionCustomisation{
			SubscriptionID: edits.SubscriptionID,
			FeedID:         state.GetFeedID(),
			UserID:         user.GetID(),
			Title:          edits.Title,
			Categories:     edits.Categories,
		}
		_, err := NewDocCreateRequest(e.GetAPI(), index, edits.SubscriptionID, customisation, refresh.True).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
		}
		return nil
	}

	updates := map[string]any{
		"title":      edits.Title,
		"categories": edits.Categories,
	}

	if err := UpdateDoc(ctx, e.GetAPI(), index, edits.SubscriptionID, updates); err != nil {
		return &models.Response{StatusCode: http.StatusInternalServerError, InternalError: err}
	}

	return nil
}

func (e *API) DeleteSubscriptionCustomisations(ctx context.Context, ids ...models.SubscriptionID) error {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return ErrFetchCtx
	}

	err := DeleteDocs(ctx, e.GetAPI(), index,
		query.Terms("subscription_id", ids...),
	)
	if err != nil {
		return fmt.Errorf("failed to delete subscription customisations: %w", err)
	}
	return nil
}

// MarkFeedUpdated updates the timestamp indicating when the feed was last updated (i.e., new items found and indexed).
func (e *API) MarkFeedUpdated(ctx context.Context, feedID models.FeedID) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	updates := map[string]any{
		"updated": time.Now().UTC(),
	}

	if err := UpdateDoc(ctx, e.GetAPI(), index, feedID, updates); err != nil {
		return fmt.Errorf("feed update failed: %w", err)
	}
	return nil
}

// UpdateUser performs a partial update of the user object. On an error, a non-nil response is returned.
func (a *API) UpdateUser(ctx context.Context, updates map[string]any) *models.Response {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.RespErrUnauthorized()
	}
	index := UserIndexFromCtx(ctx)

	if err := UpdateDoc(ctx, a.GetAPI(), index, user.GetID(), updates); err != nil {
		return &models.Response{StatusCode: http.StatusInternalServerError, InternalError: err}
	}
	return nil
}

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O models.HasID[T]](ctx context.Context, api *API, index string, objects ...O) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAPIRequestFailed, bulkOpResponse.Err)
	}
	// Create  a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](ctx context.Context, api *typedapi.API, index string, ids ...T) ([]O, error) {
	docIDs := make([]string, 0, len(ids))
	for id := range slices.Values(ids) {
		docIDs = append(docIDs, string(id))
	}

	resp, err := NewMGetRequest(api,
		GetFromIndex(index),
		GetIDs(docIDs...)).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetFailed, err)
	}
	objects, warnings := ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// UpdateDoc performs a partial doc update on the document with the given id in the given index. A non-nil error is
// returned on a failure.
func UpdateDoc[T ~string](ctx context.Context, api *typedapi.API, index string, id T, updates map[string]any) error {
	baseUpdates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if updates != nil {
		// Add the updated_at field.
		maps.Copy(updates, baseUpdates)
	} else {
		// Just update the updated_at field (i.e., `touch` the document).
		updates = baseUpdates
	}

	// Update the user in the store with the new list of read items.
	resp, err := NewDocUpdateRequest(api, index, string(id),
		WithPartialDocUpdate(updates),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Updated document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}

	return nil
}

// SearchDocs performs a _search request to find documents matching the given query.
//
// count specifies the number of results. If not specified, up to 10 results will be returned.
//
// sort specifies how to sort the resuls. If not specified, doc value sorting is used.
//
// pagination specifies the sort after values to use for getting a specific window of the total results. When set, the
// count parameter can be thought of as specifying how many new results are retrieved.
func SearchDocs[O any](ctx context.Context, api *typedapi.API, index string, query query.Option, count int, sort []types.SortCombinationsVariant, pagination *models.Pagination) ([]O, models.Pagination, error) {
	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = decodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}

	resp, err := NewSearchRequest(api,
		WithSearchID(middleware.GetReqID(ctx)),
		WithSearchIndex(index),
		WithSearchQueryOptions(query),
		WithSearchAfter(sortValues),
		WithSearchSize(count),
		WithSortOptions(sort...),
	).Do(ctx)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}

	var warnings error
	var docs []O

	docs, sortValues, warnings = ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	if pagination != nil {
		*pagination, err = encodePagination(sortValues)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
		return docs, *pagination, nil
	}

	return docs, "", nil
}

// DeleteDocs performs a delete by query request on the given index to delete documents matching the given queries.
func DeleteDocs(ctx context.Context, api *typedapi.API, index string, queries ...query.Option) error {
	resp, err := NewDeleteByQueryRequest(api, index,
		WithRequestID[*deletebyquery.DeleteByQuery, RequestCommon[*deletebyquery.DeleteByQuery]](middleware.GetReqID(ctx)),
		WithQueryOptions[*deletebyquery.DeleteByQuery, RequestWithQuery[*deletebyquery.DeleteByQuery]](queries...),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Delete documents.",
			slog.Int64("count", *resp.Deleted),
		)
	}
	return nil
}

func parseError(err error) *models.Response {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return models.NewResponse(esErr.Status, esErr)
	}
	return models.NewResponse(http.StatusInternalServerError, err)
}
