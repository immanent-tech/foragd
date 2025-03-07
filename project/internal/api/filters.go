// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/validation"
)

var (
	ErrParseFilters = errors.New("error parsing filters")
)

const (
	MaxUserCount     = 20
	MinUserCount     = 1
	DefaultUserCount = 10

	DefaultUserView = Unread
)

type ParamName string

type ParamsOption func(url.Values) url.Values

// GetFeeds gets the list of FeedIDs from the filters.
func (f *Filters) GetFeeds() []models.FeedID {
	return f.Feeds
}

// SetFeeds sets the feed filters to the given values. Existing values are wiped.
func (f *Filters) SetFeeds(feedIDs ...models.FeedID) {
	f.Feeds = feedIDs
}

// GetItems gets the list of ItemIDs from the filters.
func (f *Filters) GetItems() []models.ItemID {
	return f.Items
}

// SetItems sets the item filters to the given values. Existing values are wiped.
func (f *Filters) SetItems(itemIDs ...models.ItemID) {
	f.Items = itemIDs
}

// GetCategories gets the list of Categories from the filters.
func (f *Filters) GetCategories() models.Categories {
	return f.Categories
}

// SetCategories sets the category filters to the given values. Existing values are wiped.
func (f *Filters) SetCategories(categories ...models.Category) {
	f.Categories = categories
}

// GetCount gets the count value from the filters. If the Count outside the
// user-selectable range, the default value will be used.
func (f *Filters) GetCount() int {
	if f.Count < MinUserCount || f.Count > MaxUserCount {
		return DefaultUserCount
	}

	return f.Count
}

// GetView gets the view value from the filters.
func (f *Filters) GetView() View {
	if f.View == "" {
		return DefaultUserView
	}

	return f.View
}

// ViewRead returns a boolean indicating whether the view filter is set to "unread".
func (f *Filters) ViewUnread() bool {
	return f.GetView() == Unread
}

// ViewRead returns a boolean indicating whether the view filter is set to "read".
func (f *Filters) ViewRead() bool {
	return f.GetView() == Read
}

// ViewRead returns a boolean indicating whether the view filter is set to "all".
func (f *Filters) ViewAll() bool {
	return f.GetView() == All
}

// GetPagination gets the pagination value from the filters.
func (f *Filters) GetPagination() Pagination {
	if f.Pagination != nil {
		return *f.Pagination
	}

	return ""
}

// Params encodes the APIFilter values into a url.Values object.
func (f *Filters) Params() url.Values {
	params := make(url.Values)

	if len(f.Feeds) > 0 {
		params.Set("feeds", strings.Join(f.GetFeeds(), ","))
	}

	if len(f.Items) > 0 {
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
func (f *Filters) String() string {
	return f.Params().Encode()
}

// FiltersFromQuery takes a url.Values object and extracts the filters.
func FiltersFromQuery(values url.Values) (*Filters, error) {
	filters := &Filters{}

	if param := values.Get(string(ParamFeeds)); param != "" {
		filters.Feeds = strings.Split(param, " ")
	}

	if param := values.Get(string(ParamItems)); param != "" {
		filters.Items = strings.Split(param, " ")
	}

	if param := values.Get(string(ParamCategories)); param != "" {
		filters.Feeds = strings.Split(param, " ")
	}

	if param := values.Get(string(ParamPagination)); param != "" {
		filters.Pagination = &param
	}

	if param := values.Get(string(ParamSort)); param != "" {
		spew.Dump(param)
	}

	if param := values.Get(string(ParamView)); param != "" {
		spew.Dump(param)
		filters.View = View(param)
	}

	if param := values.Get(string(ParamCount)); param != "" {
		count, err := strconv.Atoi(param)
		if err != nil {
			return nil, errors.Join(ErrParseFilters, err)
		}
		filters.Count = count
	}

	if valid, err := validation.ValidateStruct(filters); !valid || err != nil {
		spew.Dump(valid, err)
		return nil, errors.Join(ErrParseFilters, errors.New("invalid filters"))
	}

	return filters, nil
}

// CreateFilters unmarshals the given query parameters passed to a specific route into a
// common Filters object.
func CreateFilters(params any) (*Filters, error) {
	filters := &Filters{}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, errors.Join(ErrParseFilters, err)
	}

	err = json.Unmarshal(data, filters)
	if err != nil {
		return nil, errors.Join(ErrParseFilters, err)
	}

	// Validate the count is within an appropriate range and adjust if
	// necessary.
	if filters.Count == 0 || filters.Count > 20 {
		filters.Count = 10
	}

	// Validate view value or set if necessary.
	if filters.View == "" {
		filters.View = DefaultUserView
	}

	// Validate all filters.
	if valid, err := validation.ValidateStruct(filters); !valid || err != nil {
		return nil, errors.Join(ErrParseFilters, errors.New("invalid filters"))
	}

	return filters, nil
}
