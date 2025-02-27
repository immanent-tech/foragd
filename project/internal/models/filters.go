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

	"github.com/oapi-codegen/runtime"
)

var (
	ErrGenFilters     = errors.New("error generating filters")
	ErrGetFilterValue = errors.New("error fetching filter value")
)

const (
	ParamView       ParamName = "view"
	ParamCount      ParamName = "count"
	ParamFeeds      ParamName = "feeds"
	ParamItems      ParamName = "items"
	ParamCategories ParamName = "categories"
	ParamMark       ParamName = "mark"
)

type ParamName string

type ParamsOption func(url.Values) url.Values

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

// GetFeeds gets the list of FeedIDs from the filters.
func (f *APIFilters) GetFeeds() []FeedID {
	if f.FeedIDs.IsNull() {
		return nil
	}

	feeds, err := f.FeedIDs.Get()
	if err != nil {
		return nil
	}

	return feeds
}

// SetFeeds sets the feed filters to the given values. Existing values are wiped.
func (f *APIFilters) SetFeeds(feedIDs ...FeedID) {
	f.FeedIDs.Set(feedIDs)
}

// GetItems gets the list of ItemIDs from the filters.
func (f *APIFilters) GetItems() ItemIDs {
	if f.ItemIDs.IsNull() {
		return nil
	}

	items, err := f.ItemIDs.Get()
	if err != nil {
		return nil
	}

	return items
}

// GetCategories gets the list of Categories from the filters.
func (f *APIFilters) GetCategories() Categories {
	if f.Categories.IsNull() {
		return nil
	}

	items, err := f.Categories.Get()
	if err != nil {
		return nil
	}

	return items
}

// GetCount gets the count value from the filters.
func (f *APIFilters) GetCount() int {
	return f.Count
}

// Params encodes the APIFilter values into a url.Values object.
func (f *APIFilters) Params() url.Values {
	params := make(url.Values)

	if f.FeedIDs.IsSpecified() {
		params.Set("feeds", strings.Join(f.GetFeeds(), ","))
	}

	if f.ItemIDs.IsSpecified() {
		params.Set("items", strings.Join(f.GetItems(), ","))
	}

	if f.Categories.IsSpecified() {
		params.Set("categories", strings.Join(f.GetCategories(), ","))
	}

	params.Set("view", string(f.View))

	params.Set("count", strconv.Itoa(f.Count))

	return params
}

// String returns a URL-encoded string of query parameters.
func (f *APIFilters) String() string {
	return f.Params().Encode()
}

// FiltersFromQuery takes a url.Values object and extracts the filters.
func FiltersFromQuery(values url.Values) (*APIFilters, error) {
	filters := &APIFilters{}

	var err error

	// ------------- Optional query parameter "feeds" -------------

	// var feedIDs *FeedIDs
	err = runtime.BindQueryParameter("form", true, false, "feeds", values, &filters.FeedIDs)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}

	// filters.SetFeeds(*feedIDs...)

	// ------------- Optional query parameter "categories" -------------

	err = runtime.BindQueryParameter("form", true, false, "categories", values, &filters.Categories)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}

	// ------------- Required query parameter "view" -------------

	if paramValue := values.Get("view"); paramValue != "" {
	} else {
		return nil, errors.Join(ErrGenFilters, err)
	}

	err = runtime.BindQueryParameter("form", true, true, "view", values, &filters.View)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}

	// ------------- Required query parameter "count" -------------

	if paramValue := values.Get("count"); paramValue != "" {
	} else {
		return nil, errors.Join(ErrGenFilters, err)
	}

	err = runtime.BindQueryParameter("form", true, true, "count", values, &filters.Count)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}

	return filters, nil
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

	// Validate the count is within an appropriate range and adjust if
	// necessary.
	if filters.Count == 0 || filters.Count > 20 {
		filters.Count = 10
	}

	// Validate view value or set if necessary.
	if filters.View == "" {
		filters.View = ViewUnread
	}

	return filters, nil
}
