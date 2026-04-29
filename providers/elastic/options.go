// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"

	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// HasHeader represents a request that can set a header value.
type HasHeader interface {
	*SearchRequest | *CountRequest | *GetRequest | *MGetRequest | *DeleteByQueryRequest | *UpdateRequest

	SetHeader(key string, value string)
}

// WithHeader option sets a header on the request.
func WithHeader[T HasHeader](key, value string) func(T) {
	return func(t T) {
		t.SetHeader(key, value)
	}
}

// HasIndex represents a request that can specify an index to operate on.
type HasIndex interface {
	*SearchRequest | *CountRequest | *MGetRequest | *DeleteByQueryRequest

	SetIndex(index string)
}

// WithIndex option sets the index the request will operate on.
func WithIndex[T HasIndex](index string) func(T) {
	return func(t T) {
		t.SetIndex(index)
	}
}

// HasQuery represents a request that can specify a query.
type HasQuery interface {
	*SearchRequest | *CountRequest | *DeleteByQueryRequest

	SetQueryOptions(options ...query.Option)
}

// WithQueryOptions option defines the query options that will be applied to the request.
func WithQueryOptions[T HasQuery](options ...query.Option) func(T) {
	return func(t T) {
		t.SetQueryOptions(options...)
	}
}

// HasAggregations represents a request that can specify aggregations.
type HasAggregations interface {
	*SearchRequest

	SetAggregations(aggs map[string]types.Aggregations)
}

// WithAggregations option defines aggregations to add to the request.
func WithAggregations[T HasAggregations](aggs map[string]types.Aggregations) func(T) {
	return func(t T) {
		t.SetAggregations(aggs)
	}
}

// HasSize represents a request that can define a size for number of results returned.
type HasSize interface {
	*SearchRequest

	SetSize(size int)
}

// WithSize option specifies the number of results to return. In most cases, if not specified, a default of 10 results
// is returned.
func WithSize[T HasSize](size int) func(T) {
	return func(t T) {
		t.SetSize(size)
	}
}

// HasSearchAfter represents a request that can specify search after data.
type HasSearchAfter interface {
	*SearchRequest

	SetSearchAfter(values ...types.FieldValueVariant)
}

// WithSearchAfter option specifies the search after values for paginating through results.
func WithSearchAfter[T HasSearchAfter](values ...types.FieldValueVariant) func(T) {
	return func(t T) {
		t.SetSearchAfter(values...)
	}
}

// HasSort represents a request that can sort its results.
type HasSort interface {
	*SearchRequest

	SetSort(sort ...types.SortCombinationsVariant)
}

// WithSort option specifies how the results will be sorted.
func WithSort[T HasSort](sort ...types.SortCombinationsVariant) func(T) {
	return func(t T) {
		t.SetSort(sort...)
	}
}

// WithDocSorting option is a convenience option to sort the results by doc id. This option is useful when fetching all
// results or deep pagination where sort order is irrelevant.
func WithDocSorting[T HasSort]() func(T) {
	return func(t T) {
		t.SetSort(&types.SortOptions{Doc_: types.NewScoreSort()})
	}
}

// HasFields represents a request that can filter its results by fields.
type HasFields interface {
	*SearchRequest

	SetFields(fields ...types.FieldAndFormatVariant)
}

// WithFields option specifies the fields to return in each result.
func WithFields[T HasFields](fields ...types.FieldAndFormatVariant) func(T) {
	return func(t T) {
		t.SetFields(fields...)
	}
}

// TrackHits is a boolean indicating whether to track total hits.
type TrackHits bool

func (t TrackHits) TrackHitsCaster() *types.TrackHits {
	value := types.TrackHits(t)
	return &value
}

// HasTotalHitsTracking represents a request that can track total hits.
type HasTotalHitsTracking interface {
	*SearchRequest

	SetTrackTotalHits(value bool)
}

// WithTrackTotalHits option specifies whether to track total hits for the request.
func WithTrackTotalHits[T HasTotalHitsTracking](value bool) func(T) {
	return func(t T) {
		t.SetTrackTotalHits(value)
	}
}

// HasCollapse represents a request that can collapse its results on a single field in each result.
type HasCollapse interface {
	*SearchRequest

	SetCollapseOn(field *types.FieldCollapse)
}

// WithCollapseField option specifies the field on which to collapse multiple results.
func WithCollapseField[T HasCollapse](field string) func(T) {
	return func(t T) {
		t.SetCollapseOn(&types.FieldCollapse{Field: field})
	}
}

// HasIDs represents a request that can operate against specific document IDs.
type HasIDs interface {
	*MGetRequest

	SetIDs(ids ...string)
}

// WithIDs option specifies the document IDs to operate on for this request.
func WithIDs[T HasIDs](ids ...string) func(T) {
	return func(t T) {
		t.SetIDs(ids...)
	}
}

// HasRefresh represents a request that can set a refresh state for affected shards after completion.
type HasRefresh interface {
	*UpdateRequest

	SetRefresh(refresh refresh.Refresh)
}

// WithRefresh option sets the refresh option. Note that while any value is accepted, only valid values of true, false
// or the string "waitfor" will have any effect.
func WithRefresh[T HasRefresh](value any) func(T) {
	return func(t T) {
		switch v := value.(type) {
		case bool:
			switch value {
			case true:
				t.SetRefresh(refresh.True)
			case false:
				t.SetRefresh(refresh.False)
			}
		case string:
			if v == "waitfor" {
				t.SetRefresh(refresh.Waitfor)
			}
		}
	}
}

type HasRetryOnConflict interface {
	*UpdateRequest

	SetRetryOnConflict(retries int)
}

func WithRetryOnConflict[T HasRetryOnConflict](retries int) func(T) {
	return func(t T) {
		t.SetRetryOnConflict(retries)
	}
}

type HasDocAsUpsert interface {
	*UpdateRequest

	SetDocAsUpsert(value bool)
}

func WithDocAsUpsert[T HasDocAsUpsert](value bool) func(T) {
	return func(t T) {
		t.SetDocAsUpsert(value)
	}
}

// FieldValue represents a value of a field.
type FieldValue struct {
	value any
}

// NewFieldValue converts any value into a FieldValue.
func NewFieldValue(value any) FieldValue {
	return FieldValue{value: value}
}

// FieldValueCaster is required to allow FieldValue to be used as an Elasticsearch  field value.
func (v FieldValue) FieldValueCaster() *types.FieldValue {
	switch data := v.value.(type) {
	case types.FieldValue:
		return &data
	default:
		fv := types.FieldValue(data)
		return &fv
	}
}
