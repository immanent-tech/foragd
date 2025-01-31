// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

var ErrGetFilterValue = errors.New("error fetching filter value")

func (f APIFilters) HasFeedIDs() bool {
	return f.FeedIDs.IsSpecified()
}

// GetFeedIDs retrieves the array of FeedIDs from the APIFilters.
func (f APIFilters) GetFeedIDs() FeedIDs {
	if f.FeedIDs.IsSpecified() {
		if feeds, err := f.FeedIDs.Get(); err == nil {
			return feeds
		}
	}

	return nil
}

// GetItemIDs retrieves the array of ItemIDs from the APIFilters.
func (f APIFilters) GetItemsIDs() ItemIDs {
	if f.ItemIDs.IsSpecified() {
		if items, err := f.ItemIDs.Get(); err == nil {
			return items
		}
	}

	return nil
}

// GetCategories retrieves the array of Categories from the APIFilters.
func (f APIFilters) GetCategories() Categories {
	if f.Categories.IsSpecified() {
		if categories, err := f.Categories.Get(); err == nil {
			return categories
		}
	}

	return nil
}

// GetCount retrieves the Count from the APIFilters. If the count is not found,
// a default value will be returned.
func (f APIFilters) GetCount() Count {
	if f.Count.IsSpecified() {
		if count, err := f.Count.Get(); err == nil {
			return count
		}
	}

	return 10
}

// GetState retrieves the State value. This indicates the state of objects
// that should be filtered.
func (f APIFilters) GetView() View {
	if !f.View.IsSpecified() {
		return ViewUnread
	}

	unread, err := f.View.Get()
	if err != nil {
		return ViewUnread
	}

	return unread
}

func (f APIFilters) GetPagination() (Pagination, error) {
	if !f.Pagination.IsSpecified() {
		return "", nil
	}

	encoded, err := f.Pagination.Get()
	if err != nil {
		return "", errors.Join(ErrGetFilterValue, err)
	}

	return encoded, nil
}

func (f APIFilters) SetPagination(data any) {
	switch d := data.(type) {
	case []byte:
		f.Pagination.Set(url.QueryEscape(string(d)))
	case string:
		f.Pagination.Set(d)
	}
}

func EncodePagination(decoded []byte) (Pagination, error) {
	return url.QueryEscape(string(decoded)), nil
}

func DecodePagination(encoded Pagination) ([]byte, error) {
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return nil, errors.Join(ErrGetFilterValue, err)
	}

	return []byte(decoded), nil
}

// String returns a URL-encoded string of query parameters. Only some query
// parameters are exposed this way.
func (f APIFilters) String() string {
	params := make(url.Values)

	if feeds := f.GetFeedIDs(); len(feeds) > 0 {
		params.Add("feeds", strings.Join(feeds, ","))
	}

	if items := f.GetItemsIDs(); len(items) > 0 {
		params.Add("items", strings.Join(items, ","))
	}

	if categories := f.GetCategories(); len(categories) > 0 {
		params.Add("categories", strings.Join(categories, ","))
	}

	if pagination, err := f.GetPagination(); err == nil && pagination != "" {
		params.Add("pagination", pagination)
	}

	params.Add("view", string(f.GetView()))

	params.Add("count", strconv.Itoa(f.GetCount()))

	return params.Encode()
}

// GenerateURL generates a new URL using the basePath provided with any non-zero
// filters.
func (f APIFilters) GenerateURL(basePath string) (*url.URL, error) {
	newURL, err := url.Parse(basePath)
	if err != nil {
		return nil, fmt.Errorf("cannot generate URL: %w", err)
	}

	newURL.RawQuery = f.String()

	return newURL, nil
}

// CreateFilters unmarshals the given query parameters passed to a specific route into a
// common APIFilters object.
func CreateFilters(params any) (*APIFilters, error) {
	filters := &APIFilters{}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("could not generate filters: %w", err)
	}

	slog.Debug("Marshaling params.",
		slog.Any("params", params),
		slog.String("data", string(data)))

	err = json.Unmarshal(data, filters)
	if err != nil {
		return nil, fmt.Errorf("could not generate filters: %w", err)
	}

	slog.Debug("Unmarshaling filters.",
		slog.Any("filters", filters))

	if filters.Count.IsSpecified() {
		if count, err := filters.Count.Get(); err == nil {
			if count == 0 || count > 20 {
				filters.Count.Set(10)
			}
		}
	}

	// if params.Pagination != nil {
	// 	if pagination, err := url.QueryUnescape(*params.Pagination); err != nil {
	// 		slog.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	// 	} else {
	// 		filters.Pagination = []byte(pagination)
	// 	}
	// }

	return filters, nil
}
