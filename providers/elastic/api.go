// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl
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
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/results"
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*typedapi.API
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *typedapi.API {
	return a.API
}

// GetFeeds retrieves the feeds with the given IDs.
func (a *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, ids...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", err))
	}
	return feeds, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}
	// Parse pagination to search after value.
	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = models.DecodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}
	// Parse sort filters into item sort options.
	sortOptions := newFeedSortOptions(sort)
	// Perform search.
	feeds, searchAfter, err := Search[*models.Feed](ctx, e.GetAPI(), index, query, count, sortOptions, sortValues)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = models.EncodePagination(searchAfter)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

// AddFeeds will bulk index the given feeds.
func (e *API) AddFeeds(ctx context.Context, feeds ...*models.Feed) (map[models.FeedID]*bulk.OperationResponse, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd[models.FeedID, *models.Feed](ctx, e, index, feeds...)
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
	}
	// Parse pagination into search after value.
	var sortValues []types.FieldValue
	if pagination != nil {
		var err error
		sortValues, err = models.DecodePagination(*pagination)
		if err != nil {
			return nil, "", errors.Join(ErrSearchFailed, err)
		}
	}
	// Parse sort filters into item sort options.
	sortOptions := newItemSortOptions(sort)
	// Perform search.
	items, searchAfter, err := Search[*models.Item](ctx, e.GetAPI(), index, query, count, sortOptions, sortValues)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	// Parse last search after value into pagination.
	newPagination, err := models.EncodePagination(searchAfter)
	if err != nil {
		return nil, "", errors.Join(ErrSearchFailed, err)
	}
	return items, newPagination, nil
}

// ItemsAggregation performs a search aggregation (i.e., only aggregations returned) on feed items with the given query
// options. It returns the raw search response.
func (e *API) ItemsAggregation(ctx context.Context, query query.Option, aggregations ...aggregations.Aggregation) (*search.Response, *models.Response) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, parseError(ErrFetchCtx)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](0),
		WithSortOptions[*search.Search, SearchRequest](sortByDoc()),
		WithAggregations[*search.Search, SearchRequest](aggregations...),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, parseError(err)
	}

	return resp, nil
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd[models.ItemID, *models.Item](ctx, e, index, items...)
}

// // SearchSubscriptionCustomisations will search the feeds index for feed matching the given query. Count, sort and
// // pagination values are optional.
// func (e *API) SearchSubscriptionCustomisations(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.SubscriptionCustomisations, models.Pagination, error) {
// 	index := SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return nil, "", errors.Join(ErrSearchFailed, ErrFetchCtx)
// 	}

// 	var sortValues []types.FieldValue
// 	if pagination != nil {
// 		var err error
// 		sortValues, err = models.DecodePagination(*pagination)
// 		if err != nil {
// 			return nil, "", errors.Join(ErrSearchFailed, err)
// 		}
// 	}

// 	sortOptions := []types.SortCombinations{sortByScore()}

// 	customisations, searchAfter, err := Search[*models.SubscriptionCustomisation](ctx, e.GetAPI(), index, query, count, sortOptions, sortValues)
// 	if err != nil {
// 		return nil, "", fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
// 	}

// 	if pagination != nil {
// 		*pagination, err = models.EncodePagination(searchAfter)
// 		if err != nil {
// 			return nil, "", errors.Join(ErrSearchFailed, err)
// 		}
// 		return customisations, *pagination, nil
// 	}

// 	return customisations, "", nil
// }

// // AddSubscriptionCustomisations performs a bulk add operation to add the given subscription customisations.
// func (e *API) AddSubscriptionCustomisations(ctx context.Context, customisations ...*models.SubscriptionCustomisation) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
// 	index := SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return nil, ErrFetchCtx
// 	}
// 	return BulkAdd[models.SubscriptionID, *models.SubscriptionCustomisation](ctx, e, index, customisations...)
// }

// // GetSubscriptionCustomisations retrieves the subscription customisations with the given IDs.
// func (e *API) GetSubscriptionCustomisations(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionCustomisations, error) {
// 	index := SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return nil, ErrFetchCtx
// 	}

// 	subscriptions, err := GetDocs[models.SubscriptionID, *models.SubscriptionCustomisation](ctx, e.GetAPI(), index, ids...)
// 	if err != nil {
// 		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
// 			slog.Any("warnings", err))
// 	}
// 	return subscriptions, nil
// }

// func (e *API) UpdateSubscriptionCustomisation(ctx context.Context, edits *models.SubscriptionEdit) error {
// 	// Retrieve user object.
// 	user, found := models.UserFromCtx(ctx)
// 	if !found {
// 		return models.ErrInvalidID
// 	}
// 	index := SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return ErrFetchCtx
// 	}

// 	found, err := Exists(ctx, e.GetAPI(), index, edits.SubscriptionID)
// 	if err != nil {
// 		return fmt.Errorf("failed to update subscription: %w", err)
// 	}
// 	if !found {
// 		state := user.GetSubscriptionState(edits.SubscriptionID)
// 		customisation := &models.SubscriptionCustomisation{
// 			SubscriptionID: edits.SubscriptionID,
// 			FeedID:         state.GetFeedID(),
// 			UserID:         user.GetID(),
// 			Title:          edits.Title,
// 			Categories:     edits.Categories,
// 		}
// 		err := CreateDoc(ctx, e.GetAPI(), index, edits.SubscriptionID, customisation)
// 		if err != nil {
// 			return fmt.Errorf("failed to update subscription: %w", err)
// 		}
// 		return nil
// 	}

// 	updates := map[string]any{
// 		"title":      edits.Title,
// 		"categories": edits.Categories,
// 	}

// 	if err := UpdateDoc(ctx, e.GetAPI(), index, edits.SubscriptionID, updates); err != nil {
// 		return &models.Response{StatusCode: http.StatusInternalServerError, InternalError: err}
// 	}

// 	return nil
// }

// func (a *API) CountAllUnread(ctx context.Context) (int64, error) {
// 	user, found := models.UserFromCtx(ctx)
// 	if !found {
// 		return 0, ErrNoUserCtx
// 	}
// 	index := UserIndexFromCtx(ctx)

// 	states := user.GetAllSubscriptionStatesByFeed()
// 	subscriptionQueries := make([]query.Option, 0, len(states))
// 	for _, state := range states {
// 		subscriptionQueries = append(subscriptionQueries, models.QueryUnreadItems(user, state))
// 	}
// 	query := query.Bool(
// 		query.Filter(
// 			query.Bool(
// 				query.Should(subscriptionQueries...),
// 			),
// 		),
// 	)

// 	count, err := Count(ctx, a.GetAPI(), index, query)
// 	if err != nil {
// 		return 0, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
// 	}
// 	return count, nil
// }

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

// GetUser fetches the user record from Elasticsearch.
func (e *API) GetUser(ctx context.Context, userID models.UserID) (*models.User, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrFetchCtx)
	}

	user, err := GetDoc[models.UserID, *models.User](ctx, e.GetAPI(), index, userID)
	if err != nil {
		return nil, fmt.Errorf("get user failed: %w", err)
	}

	return user, nil
}

func (e *API) FindSuggestions(ctx context.Context, searchTerms string) (results.MSearchResults, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrFetchCtx
	}

	subscriptionsQuery := &query.MsearchSearch{
		Name:  "subscriptions",
		Index: FeedsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Term("user_id", user.GetID()),
				),
				query.Should(
					query.SearchAsYouType(searchTerms, "title"),
					query.SearchAsYouType(searchTerms, "categories"),
				),
			),
		),
	}

	feedIDs := slices.Collect(maps.Values(user.GetSubscriptionsByFeedID()))
	feedsQuery := &query.MsearchSearch{
		Name:  "feeds",
		Index: FeedsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs...),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "title"),
							query.SearchAsYouType(searchTerms, "description"),
							query.SearchAsYouType(searchTerms, "content"),
							query.SearchAsYouType(searchTerms, "categories"),
						),
					),
				),
			),
		),
	}

	articlesQuery := &query.MsearchSearch{
		Name:  "items",
		Index: ItemsIndexFromCtx(ctx),
		Query: query.Build(
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs...),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(searchTerms, "title"),
							query.SearchAsYouType(searchTerms, "description"),
							query.SearchAsYouType(searchTerms, "content"),
							query.SearchAsYouType(searchTerms, "categories"),
						),
					),
				),
			),
		),
	}

	results, err := MultiSearch(ctx, e.GetAPI(), subscriptionsQuery, feedsQuery, articlesQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return results, nil
}

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O Object[T]](ctx context.Context, api *API, index string, objects ...O) (map[T]*bulk.OperationResponse, error) {
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

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkUpdate[T ~string, O Object[T]](ctx context.Context, api *API, index string, objects ...O) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.AsOperationType(bulk.BulkUpdate),
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

// Exists checks if the document with the given id exists in the given index.
func Exists[T ~string](ctx context.Context, api *typedapi.API, index string, id T) (bool, error) {
	found, err := api.Exists(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	return found, nil
}

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, api *typedapi.API, index string, queries ...query.Option) (int64, error) {
	resp, err := NewCountRequest(api,
		WithRequestID[*count.Count, CountRequest](middleware.GetReqID(ctx)),
		WithIndex[*count.Count, CountRequest](index),
		WithQueryOptions[*count.Count, CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return resp.Count, nil
}

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](ctx context.Context, api *typedapi.API, index string, ids ...T) ([]O, error) {
	docIDs := make([]string, 0, len(ids))
	for id := range slices.Values(ids) {
		docIDs = append(docIDs, string(id))
	}

	resp, err := NewMGetRequest(api,
		WithRequestID[*mget.Mget, MgetRequest](middleware.GetReqID(ctx)),
		WithIndex[*mget.Mget, MgetRequest](index),
		WithIDs[*mget.Mget, MgetRequest](docIDs...),
	).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get docs failed: %w", err)
	}
	objects, warnings := results.ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// GetDoc retrieves the doc with the given id from the given index. A non-nil error is returned on a failure.
func GetDoc[T ~string, O any](ctx context.Context, api *typedapi.API, index string, id T) (O, error) {
	var doc O
	resp, err := NewGetRequest(api, index, string(id),
		WithRequestID[*get.Get, RequestCommon[*get.Get]](middleware.GetReqID(ctx)),
	).Do(ctx)
	if err != nil {
		return doc, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](ctx context.Context, api *typedapi.API, index string, id T, doc O) error {
	resp, err := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Created document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

// UpdateDoc performs a partial doc update on the document with the given id in the given index. A non-nil error is
// returned on a failure.
func UpdateDoc[T ~string](ctx context.Context, api *typedapi.API, index string, id T, updates map[string]any, options ...Option[UpdateDocRequest]) error {
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
	resp, err := NewUpdateDocRequest(api, index, string(id), updates, options...).Do(ctx)
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

// DeleteDoc deletes the document with the given id from the given index.
func DeleteDoc[T ~string](ctx context.Context, api *typedapi.API, index string, id T) error {
	resp, err := api.Delete(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Deleted document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
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

// Search performs a _search request to find documents matching the given query.
//
// count specifies the number of results. If not specified, up to 10 results will be returned.
//
// sort specifies how to sort the resuls. If not specified, doc value sorting is used.
//
// pagination specifies the sort after values to use for getting a specific window of the total results. When set, the
// count parameter can be thought of as specifying how many new results are retrieved.
func Search[O any](ctx context.Context, api *typedapi.API, index string, query query.Option, count int, sort []types.SortCombinations, searchAfter []types.FieldValue) ([]O, []types.FieldValue, error) {
	resp, err := NewSearchRequest(api,
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](count),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter),
		WithSortOptions[*search.Search, SearchRequest](sort...),
	).Do(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("search request failed: %w", err)
	}

	var warnings error
	var docs []O

	docs, searchAfter, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).Warn("Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return docs, searchAfter, nil
}

func MultiSearch(ctx context.Context, api *typedapi.API, searches ...*query.MsearchSearch) (results.MSearchResults, error) {
	subscriptionsIndex := FeedsIndexFromCtx(ctx)
	if subscriptionsIndex == "" {
		return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}
	itemsIndex := ItemsIndexFromCtx(ctx)
	if itemsIndex == "" {
		return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	var options []Option[MsearchRequest]
	options = append(options, WithRequestID[*msearch.Msearch, MsearchRequest](middleware.GetReqID(ctx)))
	for search := range slices.Values(searches) {
		options = append(options, WithSearch(search))
	}

	resp, err := NewMSearchRequest(api, options...).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReqFailed, err)
	}

	results := make(map[string]*types.MultiSearchItem)
	for idx, search := range searches {
		if result, ok := resp.Responses[idx].(*types.MultiSearchItem); ok {
			results[search.Name] = result
		}
	}

	return results, nil
}

func parseError(err error) *models.Response {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return models.NewResponse(
			models.WithResponseStatusCode(esErr.Status),
			models.WithResponseError(esErr),
		)
	}
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusInternalServerError),
		models.WithResponseError(errors.New("unknown elastic error")),
	)
}
