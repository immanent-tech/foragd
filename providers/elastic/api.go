// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
)

// InternalPaginationCount defines the number of docs to retrieve in a pagination request.
const InternalPaginationCount = 1000

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*typedapi.API
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *typedapi.API {
	return a.API
}

// AddFeeds will bulk index the given feeds.
func (e *API) AddFeeds(ctx context.Context, feeds ...*models.Feed) (map[models.FeedID]*bulk.OperationResponse, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, feeds...)
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, items...)
}

// AddSubscriptionCustomisations performs a bulk add operation to add the given subscription customisations.
func (e *API) AddSubscriptionCustomisations(ctx context.Context, customisations ...*models.SubscriptionCustomisation) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
	index := SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, ErrFetchCtx
	}
	return BulkAdd(ctx, e, index, customisations...)
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

func UpdateDoc[T ~string](ctx context.Context, api *typedapi.API, index string, id T, updates map[string]any) error {
	// Update the user in the store with the new list of read items.
	_, err := NewDocUpdateRequest(api, index, string(id),
		WithPartialDocUpdate(updates),
	).Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
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
