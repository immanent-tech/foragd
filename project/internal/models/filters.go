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

	MaxUserCount     = 20
	MinUserCount     = 1
	DefaultUserCount = 10

	DefaultUserView = ViewUnread
)

type ParamName string

type ParamsOption func(url.Values) url.Values

// GetFeeds gets the list of FeedIDs from the filters.
func (f *APIFilters) GetFeeds() []FeedID {
	return f.FeedIDs
}

// SetFeeds sets the feed filters to the given values. Existing values are wiped.
func (f *APIFilters) SetFeeds(feedIDs ...FeedID) {
	f.FeedIDs = feedIDs
}

// GetItems gets the list of ItemIDs from the filters.
func (f *APIFilters) GetItems() ItemIDs {
	return f.ItemIDs
}

// SetItems sets the item filters to the given values. Existing values are wiped.
func (f *APIFilters) SetItems(itemIDs ...ItemID) {
	f.ItemIDs = itemIDs
}

// GetCategories gets the list of Categories from the filters.
func (f *APIFilters) GetCategories() Categories {
	return f.Categories
}

// SetCategories sets the category filters to the given values. Existing values are wiped.
func (f *APIFilters) SetCategories(categories ...Category) {
	f.Categories = categories
}

// GetCount gets the count value from the filters. If the Count outside the
// user-selectable range, the default value will be used.
func (f *APIFilters) GetCount() int {
	if f.Count < MinUserCount || f.Count > MaxUserCount {
		return DefaultUserCount
	}

	return f.Count
}

// GetView gets the view value from the filters.
func (f *APIFilters) GetView() View {
	if f.View == "" {
		return DefaultUserView
	}

	return f.View
}

// Params encodes the APIFilter values into a url.Values object.
func (f *APIFilters) Params() url.Values {
	params := make(url.Values)

	if len(f.FeedIDs) > 0 {
		params.Set("feeds", strings.Join(f.GetFeeds(), ","))
	}

	if len(f.ItemIDs) > 0 {
		params.Set("items", strings.Join(f.GetItems(), ","))
	}

	if len(f.Categories) > 0 {
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

	var (
		err        error
		feedIDs    *FeedIDs
		itemIDs    *ItemIDs
		categories *Categories
	)

	// Parse feeds param.
	err = runtime.BindQueryParameter("form", true, false, string(ParamFeeds), values, &feedIDs)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}
	// Set feeds param if present.
	if feedIDs != nil {
		filters.SetFeeds(*feedIDs...)
	}
	// Parse items param.
	err = runtime.BindQueryParameter("form", true, false, string(ParamItems), values, &itemIDs)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}
	// Set items param if present.
	if itemIDs != nil {
		filters.SetItems(*itemIDs...)
	}
	// Parse categories param.
	err = runtime.BindQueryParameter("form", true, false, string(ParamCategories), values, &categories)
	if err != nil {
		return nil, errors.Join(ErrGenFilters, err)
	}
	// Set categories param if present.
	if categories != nil {
		filters.SetCategories(*categories...)
	}
	// Set view param.
	if filters.View = View(values.Get(string(ParamView))); filters.View == "" {
		return nil, errors.Join(ErrGenFilters, err)
	}
	// Set count param.
	if paramValue := values.Get(string(ParamCount)); paramValue == "" {
		return nil, errors.Join(ErrGenFilters, err)
	} else {
		filters.Count, err = strconv.Atoi(paramValue)
		if err != nil {
			return nil, errors.Join(ErrGenFilters, err)
		}
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
