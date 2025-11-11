// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/msearch"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/go-chi/chi/v5/middleware"
	feeds "github.com/immanent-tech/go-syndication"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
	"github.com/immanent-tech/foragd/server/session/store"
)

var (
	ErrNotFound     = models.NewAPIError(errors.New("not found"), http.StatusNotFound)
	ErrNoIndexInCtx = models.NewAPIError(errors.New("no index in context"), http.StatusInternalServerError)
)

var (
	_ store.Datastore = (*API)(nil)

	_ types.FieldValueVariant = (*paginationValue[types.FieldValue])(nil)
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*elasticsearch.TypedClient
}

// GetSession retrieves session data with the given token.
func (a *API) GetSession(ctx context.Context, token string) (*models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	session, err := GetDoc[string, models.UserSession](ctx, a.GetAPI(), index, token)
	if err != nil {
		return nil, toAPIError(err)
	}
	return &session, nil
}

// DeleteSession removes the session data for the given token.
func (a *API) DeleteSession(ctx context.Context, token string) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	err := DeleteDoc(ctx, a.GetAPI(), index, token)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// UpdateSession updates the session data.
func (a *API) UpdateSession(ctx context.Context, token string, data map[string]any) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	err := UpdateDoc(ctx, a.GetAPI(), index,
		token,
		data,
		UpdateDocAsUpsert(),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// FindAllSessions returns all active (non-expired) sessions.
func (a *API) FindAllSessions(ctx context.Context) ([]models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	sessions, err := SearchAll[models.UserSession](ctx, a.GetAPI(), index, query.Since("expiry", time.Now().UTC()), 1000)
	if err != nil {
		return nil, toAPIError(err)
	}
	return sessions, nil
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *elasticsearch.TypedClient {
	return a.TypedClient
}

// UserExists checks if a user with the given ID exists.
func (a *API) UserExists(ctx context.Context, id models.UserID) (bool, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return false, ErrNoIndexInCtx
	}
	found, err := exists(ctx, a.TypedClient, index, id)
	if err != nil {
		return false, toAPIError(err)
	}
	return found, nil
}

func (a *API) CreateUser(ctx context.Context, user *models.User) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	err = CreateDoc(ctx, a.GetAPI(), index, user.GetID(), user)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// GetUser retrieves the user with the given id.
func (a *API) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	user, err := GetDoc[models.UserID, *models.User](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return user, nil
}

func (a *API) DeleteUser(ctx context.Context, id models.UserID) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// UpdateUser will apply the given updates to the user.
func (a *API) UpdateUser(ctx context.Context, userID models.UserID, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	err = UpdateDoc(ctx, a.GetAPI(), index, userID, updates,
		WithRefresh("true"),
		WithRetryOnConflict(5),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// FindUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (a *API) FindUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	// Get the user.
	users, _, err := Search[*models.User](ctx, a.GetAPI(), index, query.Term("external_user_id", externalID), 1)
	if err != nil {
		return nil, toAPIError(err)
	}
	if len(users) == 0 {
		return nil, ErrNotFound
	}
	return users[0], nil
}

// GetFeed retrieves a single feed with the given ID.
func (a *API) GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	feed, err := GetDoc[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feed, nil
}

func (a *API) CreateFeed(ctx context.Context, feed *models.Feed) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	err = CreateDoc(ctx, a.GetAPI(), index, feed.GetID(), feed)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

func (a *API) DeleteFeed(ctx context.Context, id models.FeedID) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	// Delete the feed.
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// GetFeeds retrieves the feeds with the given IDs.
func (a *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, ids...)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feeds, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchFeeds(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Feeds, models.Pagination, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", ErrNoIndexInCtx
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(fmt.Errorf("decode pagination failed: %w", err), http.StatusInternalServerError)
	}

	// Perform search.
	feeds, newSearchAfter, err := Search[*models.Feed](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](models.NewFeedSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", toAPIError(err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", models.NewAPIError(fmt.Errorf("encode pagination failed: %w", err), http.StatusInternalServerError)
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

func (e *API) MultiSearchFeeds(ctx context.Context, queries ...*models.MultiSearchQuery) (results.MSearchResults, error) {
	return MultiSearch(ctx, e.GetAPI(), queries...)
}

// GetNewFeedsSince will return a slice of all feeds that have been created since the given timestamp.
func (e *API) GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error) {
	// Get all new feeds created since last checkpoint.
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	// Generate query. We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	query := query.Term("last_fetched", models.UnixEpoch)
	var feeds models.Feeds
	feeds, err = SearchAll[*models.Feed](ctx, e.GetAPI(), index, query, 1000)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feeds, nil
}

// UpdateFeed will update the feed with the given id, using the new feed information provided.
func (e *API) UpdateFeed(ctx context.Context, id models.FeedID, updated *feeds.Feed) error {
	// Update the feed timestamp.
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	updates := map[string]any{
		"last_fetched": time.Now().UTC(),
	}
	err = UpdateDoc(ctx, e.GetAPI(), index, id, updates)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchItems(ctx context.Context, query query.Option, count int, sort *models.Sort, pagination *models.Pagination) (models.Items, models.Pagination, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", ErrNoIndexInCtx
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(fmt.Errorf("decode pagination failed: %w", err), http.StatusInternalServerError)
	}
	// Perform search.
	items, newSearchAfter, err := Search[*models.Item](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](models.NewItemSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", toAPIError(err)
	}
	// Parse last search after value into pagination.
	newPagination, err := encodePagination(newSearchAfter)
	if err != nil {
		return nil, "", models.NewAPIError(fmt.Errorf("encode pagination failed: %w", err), http.StatusInternalServerError)
	}
	return items, newPagination, nil
}

func (e *API) ItemsAggregation(ctx context.Context, query query.Option, size int, aggregations aggregations.Aggs) (*search.Response, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, toAPIError(err)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](size),
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithAggregations2[*search.Search, SearchRequest](aggregations),
	)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, toAPIError(err)
	}

	return resp, nil
}

// CountItems returns a count of items that match the given query.
func (e *API) CountItems(ctx context.Context, query query.Option) (int64, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return 0, ErrNoIndexInCtx
	}

	count, err := Count(ctx, e.GetAPI(), index, query)
	if err != nil {
		return 0, toAPIError(err)
	}

	return count, nil
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index, err := ItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	return BulkUpdate(ctx, e, index, items...)
}

// ArchiveArticle will index the given article content to the article archive for permanent storage.
func (a *API) ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	index, err := FavoriteItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	err = CreateDoc(ctx, a.GetAPI(), index, article.ItemID, article)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// UnarchiveArticle will delete an article from the archive.
func (a *API) UnarchiveArticle(ctx context.Context, userID models.UserID, itemID models.ItemID) error {
	index, err := FavoriteItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", userID),
			query.Term("item_id", itemID),
		),
	)
	err = DeleteDocs(ctx, a.GetAPI(), index, query)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

func (a *API) GetJobState(ctx context.Context, id string) (*models.JobState, error) {
	index, err := SchedulerReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx
	}
	state, err := GetDoc[string, *models.JobState](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return state, nil
}

func (a *API) UpdateJobState(ctx context.Context, id string, updates map[string]any) error {
	index, err := SchedulerWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx
	}
	updates["updated_at"] = time.Now().UTC()
	err = UpdateDoc(ctx, a.GetAPI(), index, id, updates,
		UpdateDocAsUpsert(),
		WithRefresh("true"),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// CountItems returns a count of items that match the given query.
func (e *API) CountJobs(ctx context.Context) (int64, error) {
	index, err := SchedulerReadIndexFromCtx(ctx)
	if err != nil {
		return 0, ErrNoIndexInCtx
	}
	count, err := Count(ctx, e.GetAPI(), index, query.Exists("job_type"))
	if err != nil {
		return 0, toAPIError(err)
	}

	return count, nil
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
		return nil, fmt.Errorf("bulk operation failed: %w", bulkOpResponse.Err)
	}
	// Create a map of responses by object id.
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

// BulkUpdate will update documents for the given list of objects. Responses are returned as a map of doc id to response.
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
				bulk.Upsert(true),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, fmt.Errorf("bulk operation failed: %w", bulkOpResponse.Err)
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

// exists checks if the document with the given id exists in the given index.
func exists[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (bool, error) {
	found, err := api.Exists(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Do(ctx)
	if err != nil {
		return false, err
	}
	return found, nil
}

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) (int64, error) {
	resp, err := NewCountRequest(api,
		WithRequestID[*count.Count, CountRequest](middleware.GetReqID(ctx)),
		WithIndex[*count.Count, CountRequest](index),
		WithQueryOptions[*count.Count, CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, err
	}

	return resp.Count, nil
}

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, ids ...T) ([]O, error) {
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
		return nil, err
	}
	objects, warnings := results.ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// GetDoc retrieves the doc with the given id from the given index. A non-nil error is returned on a failure.
func GetDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (O, error) {
	var doc O
	resp, err := NewGetRequest(api, index, string(id),
		WithRequestID[*get.Get, RequestCommon[*get.Get]](middleware.GetReqID(ctx)),
	).Do(ctx)
	if err != nil {
		return doc, err
	}
	if !resp.Found {
		return doc, ErrNotFound
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, models.NewAPIError(fmt.Errorf("extract doc failed: %w", err), http.StatusInternalServerError)
	}
	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T, doc O) error {
	resp, err := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return err
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
func UpdateDoc[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T, updates map[string]any, options ...Option[UpdateDocRequest]) error {
	resp, err := NewUpdateDocRequest(api, index, string(id), updates, options...).Do(ctx)
	if err != nil {
		return err
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
func DeleteDoc[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) error {
	resp, err := api.Delete(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return err
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
func DeleteDocs(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) error {
	resp, err := NewDeleteByQueryRequest(api, index,
		WithRequestID[*deletebyquery.DeleteByQuery, RequestCommon[*deletebyquery.DeleteByQuery]](middleware.GetReqID(ctx)),
		WithQueryOptions[*deletebyquery.DeleteByQuery, RequestWithQuery[*deletebyquery.DeleteByQuery]](queries...),
	).Do(ctx)
	if err != nil {
		return err
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Delete documents.",
			slog.Int64("count", *resp.Deleted),
		)
	}
	return nil
}

// Search performs a _search request to find documents matching the given query.
func Search[O any](ctx context.Context, api *elasticsearch.TypedClient, index string, query query.Option, count int, options ...Option[SearchRequest]) ([]O, []types.FieldValue, error) {
	defaultOptions := []Option[SearchRequest]{
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](count),
	}
	defaultOptions = append(defaultOptions, options...)
	req := NewSearchRequest(api, defaultOptions...)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, err
	}
	var warnings error
	var docs []O
	var newSearchAfter []types.FieldValue

	docs, newSearchAfter, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return docs, newSearchAfter, nil
}

// SearchAll performs a paginated search request to retrieve *all* documents matching the given query. Unlike Search, it
// does not stop when the request hits count is reached.
func SearchAll[O any](ctx context.Context, api *elasticsearch.TypedClient, index string, query query.Option, paginationSize int, options ...Option[SearchRequest]) ([]O, error) {
	if paginationSize == 0 {
		paginationSize = 1000
	}
	allResults := make([]O, 0)
	var searchAfter []types.FieldValueVariant

	// Loop until we've paginated through all results.
	var loops int
	for {
		resultsPage, nextSearchAfter, err := Search[O](ctx, api, index, query, paginationSize,
			WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
			WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
		)
		if err != nil {
			return nil, err
		}
		pagination, err := encodePagination(nextSearchAfter)
		if err != nil {
			return nil, models.NewAPIError(fmt.Errorf("encode pagination failed: %w", err), http.StatusInternalServerError)
		}
		searchAfter, err = decodePagination(&pagination)
		if err != nil {
			return nil, models.NewAPIError(fmt.Errorf("decode pagination failed: %w", err), http.StatusInternalServerError)
		}

		allResults = append(allResults, resultsPage...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resultsPage) < paginationSize {
			break
		}
		loops++
	}
	slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Paginated search finished.",
		slog.Int("loops", loops),
	)
	return allResults, nil
}

// MultiSearch performs an msearch request.
func MultiSearch(ctx context.Context, api *elasticsearch.TypedClient, searches ...*models.MultiSearchQuery) (results.MSearchResults, error) {
	// subscriptionsIndex, err := FeedsReadIndexFromCtx(ctx)
	// if err != nil {
	// 	return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
	// }
	// itemsIndex, err := ItemsReadIndexFromCtx(ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("unable to perform multi-search: %w", err)
	// }

	options := make([]Option[MsearchRequest], 0, len(searches)+1)
	options = append(options, WithRequestID[*msearch.Msearch, MsearchRequest](middleware.GetReqID(ctx)))
	for search := range slices.Values(searches) {
		options = append(options, WithSearch(search))
	}

	req := NewMSearchRequest(api, options...)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	results := make(map[string]*types.MultiSearchItem)
	for idx, search := range searches {
		if result, ok := resp.Responses[idx].(*types.MultiSearchItem); ok {
			results[search.Name] = result
		}
	}

	return results, nil
}

func toAPIError(err error) error {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		msg := fmt.Errorf("%s: %s", esErr.ErrorCause.Type, *esErr.ErrorCause.Reason)
		return models.NewAPIError(msg, esErr.Status)
	}
	return models.NewAPIError(err, http.StatusInternalServerError)
}

// paginationValue is a value that can be used as a sort value as a search after option.
type paginationValue[T types.FieldValue] struct {
	value T
}

func newPaginationValue[T any](value T) *paginationValue[T] {
	return &paginationValue[T]{value: value}
}

func (v *paginationValue[T]) FieldValueCaster() *types.FieldValue {
	casted := types.FieldValue(v)
	return &casted
}

func (v *paginationValue[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(v.value)
	if err != nil {
		return data, fmt.Errorf("failed to marshal pagination value: %w", err)
	}
	return data, nil
}

// encodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a models.Pagination object, which is
// safe for use in API query parameters.
func encodePagination(sortValues []types.FieldValue) (models.Pagination, error) {
	if len(sortValues) == 0 {
		return "", nil
	}
	// Marshal sort values into json.
	data, err := json.Marshal(sortValues)
	if err != nil {
		return "", fmt.Errorf("could not encode pagination values: %w", err)
	}
	// Return as HTML encoded string.
	return url.QueryEscape(string(data)), nil
}

// decodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func decodePagination(pagination *models.Pagination) ([]types.FieldValueVariant, error) {
	if pagination == nil {
		return nil, nil
	}
	if *pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(*pagination)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	// Unmarshal sort values.
	var values []any
	err = json.Unmarshal([]byte(data), &values)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	casted := make([]types.FieldValueVariant, 0, len(values))
	for v := range slices.Values(values) {
		switch r := v.(type) {
		case string:
			casted = append(casted, newPaginationValue(r))
		case int64:
			casted = append(casted, newPaginationValue(r))
		case float64:
			casted = append(casted, newPaginationValue(r))
		default:
			casted = nil
		}
	}

	// Return sort values.
	return casted, nil
}
