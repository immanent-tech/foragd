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

	DefaultUserView = ViewUnread
)

type ParamsOption func(url.Values) url.Values

// Sort Feeds by last updated, newest->oldest.
var SortFeedsLastUpdatedDesc = Sort{SortBy: LastUpdated, SortOrder: SortDesc}

// Sort Feeds by last updated, oldest->newest.
var SortFeedsLastUpdatedAsc = Sort{SortBy: LastUpdated, SortOrder: SortAsc}

// Sort Feeds by unread count, highest->lowest.
var SortFeedsUnreadCountDesc = Sort{SortBy: UnreadCount, SortOrder: SortDesc}

// Sort Feeds by unread count, lowest->highest.
var SortFeedsUnreadCountAsc = Sort{SortBy: UnreadCount, SortOrder: SortAsc}

// Valid checks whether the Sort options are valid values.
func (s *Sort) Valid() bool {
	valid, err := validation.ValidateStruct(s)
	if !valid || err != nil {
		return false
	}
	return true
}

// GetFeeds gets the list of FeedIDs from the filters.
func (f *Filters) GetFeeds() []models.FeedID {
	if f.Feeds != nil {
		return *f.Feeds
	}
	return nil
}

// SetFeeds sets the feed filters to the given values. Existing values are wiped.
func (f *Filters) SetFeeds(feedIDs ...models.FeedID) {
	feeds := make([]models.FeedID, len(feedIDs))
	feeds = append(feeds, feedIDs...)
	f.Feeds = &feeds
}

// GetItems gets the list of ItemIDs from the filters.
func (f *Filters) GetItems() []models.ItemID {
	if f.Items != nil {
		return *f.Items
	}
	return nil
}

// SetItems sets the item filters to the given values. Existing values are wiped.
func (f *Filters) SetItems(itemIDs ...models.ItemID) {
	items := make([]models.ItemID, len(itemIDs))
	items = append(items, itemIDs...)
	f.Items = &items
}

// GetCategories gets the list of Categories from the filters.
func (f *Filters) GetCategories() models.Categories {
	if f.Categories != nil {
		return *f.Categories
	}
	return nil
}

// SetCategories sets the category filters to the given values. Existing values are wiped.
func (f *Filters) SetCategories(categories ...models.Category) {
	c := make([]models.Category, len(categories))
	c = append(c, categories...)
	f.Categories = &c
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
	return f.GetView() == ViewUnread
}

// ViewRead returns a boolean indicating whether the view filter is set to "read".
func (f *Filters) ViewRead() bool {
	return f.GetView() == ViewRead
}

// ViewRead returns a boolean indicating whether the view filter is set to "all".
func (f *Filters) ViewAll() bool {
	return f.GetView() == ViewAll
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

	// if len(f.Feeds) > 0 {
	if f.Feeds != nil {
		params.Set("feeds", strings.Join(f.GetFeeds(), ","))
	}

	// if len(f.Items) > 0 {
	if f.Items != nil {
		params.Set("items", strings.Join(f.GetItems(), ","))
	}

	// if len(f.Categories) > 0 {
	if f.Categories != nil {
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
		feeds := strings.Split(param, " ")
		filters.Feeds = &feeds
	}

	if param := values.Get(string(ParamItems)); param != "" {
		items := strings.Split(param, " ")
		filters.Items = &items
	}

	if param := values.Get(string(ParamCategories)); param != "" {
		categories := strings.Split(param, " ")
		filters.Categories = &categories
	}

	if param := values.Get(string(ParamPagination)); param != "" {
		filters.Pagination = &param
	}

	if param := values.Get(string(ParamSortBy)); param != "" {
		spew.Dump(param)
	}

	if param := values.Get(string(ParamView)); param != "" {
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
