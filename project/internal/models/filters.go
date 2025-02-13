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

type ParamsOption func(url.Values) url.Values

var ErrGetFilterValue = errors.New("error fetching filter value")

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

// Params encodes the APIFilter values into a url.Values object.
func (f APIFilters) Params() url.Values {
	params := make(url.Values)

	if len(f.FeedIDs) > 0 {
		params.Set("feeds", strings.Join(f.FeedIDs, ","))
	}

	if len(f.ItemIDs) > 0 {
		params.Set("items", strings.Join(f.ItemIDs, ","))
	}

	if len(f.Categories) > 0 {
		params.Set("categories", strings.Join(f.Categories, ","))
	}

	params.Set("view", string(f.View))

	params.Set("count", strconv.Itoa(f.Count))

	return params
}

// String returns a URL-encoded string of query parameters.
func (f APIFilters) String() string {
	return f.Params().Encode()
}

// WithFeeds option replaces any existing FeedID filters with the given list.
func WithFeeds(ids ...FeedID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("feeds", strings.Join(ids, ","))
		return v
	}
}

// WithItems option replaces any existing ItemID filters with the given list.
func WithItems(ids ...ItemID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("items", strings.Join(ids, ","))
		return v
	}
}

// WithItems option replaces any existing ItemID filters with the given list.
func WithCategories(categories ...Category) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("categories", strings.Join(categories, ","))
		return v
	}
}

func WithView(view View) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("view", string(view))
		return v
	}
}

// ExcludeFeeds option removes any FeedID filters from the params.
func ExcludeFeeds() ParamsOption {
	return func(v url.Values) url.Values {
		v.Del("feeds")
		return v
	}
}

// ExcludeItems option removes any ItemID filters from the params.
func ExcludeItems() ParamsOption {
	return func(v url.Values) url.Values {
		v.Del("items")
		return v
	}
}

// ExcludeFeeds option removes any Category filters from the params.
func ExcludeCategories() ParamsOption {
	return func(v url.Values) url.Values {
		v.Del("categories")
		return v
	}
}

// BuildURL will generate a url.URL object from the given path and the current
// APIFilters, with an exclusions specified as ParamsOption.
func (f APIFilters) BuildURL(path string, options ...ParamsOption) *url.URL {
	newURL, err := url.Parse(path)
	if err != nil {
		return nil
	}

	params := f.Params()

	for _, option := range options {
		params = option(params)
	}

	newURL.RawQuery = params.Encode()

	return newURL
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

	if filters.Count == 0 || filters.Count > 20 {
		filters.Count = 10
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
