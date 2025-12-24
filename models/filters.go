// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/immanent-tech/foragd/validation"
)

func init() {
	gob.Register(ListFilters{})
}

var ErrNoFilters = &APIError{
	InternalError: errors.New("not filters found"),
	StatusCode:    http.StatusNotFound,
}

const (
	// maxUserCount is the maximum number of results a user can retrieve at a single time.
	maxUserCount = 45
	// minUserCount is the mininum number of results a user can retrieve at a single time.
	minUserCount = 1
	// defaultCount is to show 10 objects.
	defaultCount    = "15"
	defaultCountInt = 15
	// defaultView is to show unread objects.
	defaultView = ViewUnread
)

// defaultSort is newest first.
var defaultSort = SortNewestFirst

func (s Sort) String() string {
	switch s {
	case SortLeastUnread:
		return "Least Unread"
	case SortMostUnread:
		return "Most Unread"
	case SortNewestFirst:
		return "Newest First"
	case SortOldestFirst:
		return "Oldest First"
	case SortMostRelevant:
		return "Most Relevant"
	default:
		return "Unknown"
	}
}

// Filters represents either Subscription or Article filters.
type Filters interface {
	Valid() error
	GetSort() Sort
	GetCount() int
	GetView() View
	GetCategories() Categories
	Values() map[string]any
	QueryString() string
}

// NewListDisplayFilters creates a new set of display filters with sensible defaults.
func NewListDisplayFilters() ListFilters {
	return ListFilters{
		Sort:  SortNewestFirst,
		Count: defaultCount,
		View:  defaultView,
	}
}

// GetSubscriptions retrieves any subscription filters.
func (f *ListFilters) GetSubscriptions() []SubscriptionID {
	return f.Subscriptions
}

// Sanitise performs sanitisation of the filter values to ensure correctness.
func (f *ListFilters) Sanitise() error {
	if f == nil {
		return ErrNoFilters
	}
	// Set required filters to valid values as necessary.
	f.Sort = setValidSort(f.Sort)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	return nil
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *ListFilters) Valid() error {
	if f == nil {
		return ErrNoFilters
	}
	if err := validation.Validate.Struct(f); err != nil {
		return fmt.Errorf("filters are invalid: %w", err)
	}
	return nil
}

// GetSort returns the Sort object for the Filters.
func (f *ListFilters) GetSort() Sort {
	return f.Sort
}

// GetCount returns the count value (encoded as a string in the filters) as an int.
func (f *ListFilters) GetCount() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return defaultCountInt
	}
	return value
}

// GetView returns the view filter.
func (f *ListFilters) GetView() View {
	return f.View
}

// GetCategories returns any category filters.
func (f *ListFilters) GetCategories() Categories {
	return f.Categories
}

// QueryParams converts the filters into query parameters.
func (f *ListFilters) QueryParams() url.Values {
	params := make(url.Values)
	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}
	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}
	params.Set(ParamSort, string(f.Sort))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)
	return params
}

// QueryString converts the filters into a string that can be appended to a URL to represent the filters.
func (f *ListFilters) QueryString() string {
	return f.QueryParams().Encode()
}

// Values converts the filters into a map[string]string object, that can be further manipulated before being (most
// likely) used as the value of hx-vals in a HTMX request.
func (f *ListFilters) Values() map[string]any {
	params := make(map[string]any)
	if len(f.Subscriptions) > 0 {
		params[ParamSubscriptions] = f.Subscriptions
	}
	if len(f.Categories) > 0 {
		params[ParamCategories] = strings.Join(f.Categories, ",")
	}
	params[ParamSort] = string(f.Sort)
	params[ParamView] = string(f.View)
	params[ParamCount] = f.Count
	return params
}

// setValidSortBy takes a string value and returns the SortBy value it
// represents. If the string is not a valid SortBy value, the default SortBy
// value is returned.
func setValidSort(value Sort) Sort {
	switch value {
	case SortLeastUnread, SortMostUnread, SortNewestFirst, SortOldestFirst, SortMostRelevant:
		return value
	default:
		return defaultSort
	}
}

// setValidCount takes a value representing a count and returns a valid Count it
// represents. If the value is not a valid Count, the default Count is
// returned.
func setValidCount(value Count) Count {
	numeric, err := strconv.Atoi(value)
	if err != nil {
		return defaultCount
	}
	if numeric < minUserCount || numeric > maxUserCount {
		return defaultCount
	}
	return value
}

// setValidView takes a string representing a View and returns a valid View it
// represents. If the value is not a valid View, the default View is
// returned.
func setValidView(value View) View {
	switch value {
	case ViewAll:
		return ViewAll
	case ViewRead:
		return ViewRead
	case ViewUnread:
		return ViewUnread
	default:
		return defaultView
	}
}

// setValidMark takes a string representing a Mark and returns a valid Mark it
// represents. If the value is not a valid Mark, it returns MarkRead.
func setValidMark(value Mark) Mark {
	switch value {
	case MarkRead:
		return MarkRead
	case MarkUnread:
		return MarkUnread
	default:
		return MarkRead
	}
}
