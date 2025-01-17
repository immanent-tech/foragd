// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	_ models.FeedManagementAPI = (*Client)(nil)
	_ models.FeedJobStateAPI   = (*Client)(nil)
	_ models.UserActionsAPI    = (*Client)(nil)
	_ models.UserManagementAPI = (*Client)(nil)
)

var (
	ErrNoClient      = errors.New("no client")
	ErrFieldNotFound = errors.New("field not found")
)

// Option is a generic type for request options.
type Option[T any] func(T) T

type hasIndexPatternOption[T any] interface {
	Index(value string) T
}

// WithValue allows setting an value on a component.
func WithIndexPattern[T any](value string) Option[T] {
	return func(req T) T {
		if settable, ok := any(req).(hasIndexPatternOption[T]); ok {
			req = settable.Index(value)
		}

		return req
	}
}

type docIDOption struct {
	id string
}

func (o *docIDOption) SetDocID(id string) {
	o.id = id
}

func (o *docIDOption) GetDocID() string {
	return o.id
}

type hasDocIDOption[T any] interface {
	SetDocID(id string)
}

// WithDocID allows setting the document ID as an option.
func WithDocID[T any](id string) Option[T] {
	return func(req T) T {
		request := &req

		if settable, ok := any(request).(hasDocIDOption[T]); ok {
			settable.SetDocID(id)
		}

		return *request
	}
}

// extractSources loops through the given hits array and extracts the `_source`
// field of each document as type `T`, returning the documents as an array
// `[]T`. Any errors extracting sources will be logged at the WARN level.
//
//nolint:prealloc
func extractSources[T any](ctx context.Context, hits []types.Hit) []T {
	var items []T

	for _, hit := range hits {
		source, err := extractSource[T](hit.Source_)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not unmarshal item source.",
				slog.Any("error", err))
			continue
		}

		items = append(items, source)
	}

	return items
}

// extractSource extracts the `_source` field from a hit. A non-nil error is
// returned if the source cannot be extracted.
func extractSource[T any](doc json.RawMessage) (T, error) {
	var source T

	if err := json.Unmarshal(doc, &source); err != nil {
		return source, errors.Join(ErrExtractSource, err)
	}

	return source, nil
}

// extractFieldValue extracts the value of the given field from a hit's list of
// returned fields. If the field is not found or the value cannot be extracted,
// a non-nil error is returned.
func extractFieldValue[T any](field string, fields map[string]json.RawMessage) (T, error) {
	var fieldValue T

	if _, found := fields[field]; !found {
		return fieldValue, ErrFieldNotFound
	}

	err := json.Unmarshal(fields[field], &fieldValue)
	if err != nil {
		return fieldValue, errors.Join(ErrFieldNotFound, err)
	}

	return fieldValue, nil
}
