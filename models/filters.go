// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/joshuar/go-feed-me/components/validation"
)

var ErrParseFilters = errors.New("error parsing filters")

var (
	// Sort by last updated, newest->oldest.
	SortLastUpdatedDesc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderDesc}
	// Sort by last updated, oldest->newest.
	SortLastUpdatedAsc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderAsc}
	// Sort by unread count, highest->lowest.
	SortUnreadCountDesc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderDesc}
	// Sort by unread count, lowest->highest.
	SortUnreadCountAsc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderAsc}
)

const (
	// MaxUserCount is the maximum number of results a user can retrieve at a single time.
	MaxUserCount = 20
	// MinUserCount is the mininum number of results a user can retrieve at a single time.
	MinUserCount = 1
	// DefaultCount is to show 10 objects.
	DefaultCount = "10"
	// DefaultView is to show unread objects.
	DefaultView = ViewUnread
	// DefaultSortBy is to sort on updated.
	DefaultSortBy = SortByLastUpdated
	// DefaultSortOrder is to sort newest->oldest.
	DefaultSortOrder = SortOrderDesc
	// DefaultSince is maximum duration (approx 290 years).
	DefaultSince = math.MaxInt64
)

func (sb SortBy) String() string {
	switch sb {
	case SortByLastUpdated:
		return "Last Updated"
	case SortByUnreadCount:
		return "Unread Count"
	default:
		return ""
	}
}

func (so SortOrder) String() string {
	switch so {
	case SortOrderAsc:
		return "Asc"
	case SortOrderDesc:
		return "Desc"
	default:
		return ""
	}
}

func (s *Sort) String() string {
	return s.SortBy.String() + ": " + s.SortOrder.String()
}

func (s Sort) IsEqual(value Sort) bool {
	return s.SortBy == value.SortBy && s.SortOrder == value.SortOrder
}

// Valid checks whether the Sort options are valid values.
func (s *Sort) Valid() bool {
	valid, err := validation.ValidateStruct(s)
	if !valid || err != nil {
		return false
	}
	return true
}

// setValidSortBy takes a string value and returns the SortBy value it
// represents. If the string is not a valid SortBy value, the default SortBy
// value is returned.
func setValidSortBy(value SortBy) SortBy {
	switch value {
	case SortByUnreadCount:
		return value
	case SortByLastUpdated:
		return value
	default:
		return DefaultSortBy
	}
}

// setValidSortOrder takes a string value and returns the SortOrder value it
// represents. If the string is not a valid SortOrder value, the default SortOrder
// value is returned.
func setValidSortOrder(value SortOrder) SortOrder {
	switch value {
	case SortOrderAsc:
		return value
	case SortOrderDesc:
		return value
	default:
		return DefaultSortOrder
	}
}

// setValidCount takes a value representing a count and returns a valid Count it
// represents. If the value is not a valid Count, the default Count is
// returned.
func setValidCount(value Count) Count {
	numeric, err := strconv.Atoi(value)
	if err != nil {
		return DefaultCount
	}
	if numeric < MinUserCount || numeric > MaxUserCount {
		return DefaultCount
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
		return DefaultView
	}
}

// Filters represents either Subscription or Article filters.
type Filters interface {
	Valid() (bool, error)
	GetSort() Sort
	GetCount() int
	ToQueryParams() url.Values
	IsSorted(sort Sort) bool
	SetSort(sort Sort)
	HasCategory(category Category) bool
	// AddCategory(category Category)
	// RemoveCategory(category Category)
	IsView(view View) bool
	SetView(view View)
	GetView() View
}

func FiltersFromParams[F Filters](params any) (F, error) {
	var filters F
	// Marshal params to JSON.
	data, err := json.Marshal(params)
	if err != nil {
		return filters, fmt.Errorf("unable to marshal params: %w", err)
	}
	// Unmarshal JSON to filters.
	err = json.Unmarshal(data, &filters)
	if err != nil {
		return filters, fmt.Errorf("unable to unmarshal params: %w", err)
	}

	valid, err := filters.Valid()
	if !valid || err != nil {
		return filters, fmt.Errorf("invalid filters: %w", err)
	}

	return filters, nil
}

func NewSubscriptionFilters() SubscriptionFilters {
	return SubscriptionFilters{
		SortBy:    SortByUnreadCount,
		SortOrder: SortOrderDesc,
		Count:     DefaultCount,
		View:      DefaultView,
	}
}

// FiltersValidation is a custom struct-level validation function for Filters.
// In this case, we validate that either a list of Feeds or Items has been
// provided, and fail validation if both have been provided.
func SubscriptionFiltersValidation(sl validator.StructLevel) {
	filters := sl.Current().Interface().(SubscriptionFilters)
	if len(filters.Subscriptions) > 0 && len(filters.Categories) > 0 {
		sl.ReportError(filters.Subscriptions, "subscriptions", "Subscriptions", "subscriptionsorcategories", "")
		sl.ReportError(filters.Categories, "categories", "categories", "subscriptionsorcategories", "")
	}
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *SubscriptionFilters) Valid() (bool, error) {
	// Register custom struct-level validation function.
	validation.AddStructValidationFunc(SubscriptionFiltersValidation, SubscriptionFilters{})
	// Set required filters to valid values as necessary.
	f.SortBy = setValidSortBy(f.SortBy)
	f.SortOrder = setValidSortOrder(f.SortOrder)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	// Validate struct.
	return validation.ValidateStruct(f)
}

// Sort returns the Sort object for the Filters.
func (f SubscriptionFilters) GetSort() Sort {
	return Sort{
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
	}
}

// CountAsInt returns the count value (encoded as a string in the filters) as an int.
func (f SubscriptionFilters) GetCount() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return 10
	}
	return value
}

func (f SubscriptionFilters) ToQueryParams() url.Values {
	params := make(url.Values)

	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}

	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}

	if f.Pagination != nil {
		params.Set(ParamPagination, *f.Pagination)
	}

	params.Set(ParamSortBy, string(f.SortBy))
	params.Set(ParamSortOrder, string(f.SortOrder))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)

	return params
}

func (f SubscriptionFilters) IsSorted(sort Sort) bool {
	return f.SortBy == sort.SortBy && f.SortOrder == sort.SortOrder
}

func (f *SubscriptionFilters) SetSort(sort Sort) {
	f.SortBy = sort.SortBy
	f.SortOrder = sort.SortOrder
}

func (f SubscriptionFilters) HasCategory(category Category) bool {
	return slices.Contains(f.Categories, category)
}

// func (f *SubscriptionFilters) AddCategory(category Category) {
// 	f.Categories = append(f.Categories, category)
// 	f.Categories = slices.Compact(f.Categories)
// }

// func (f *SubscriptionFilters) RemoveCategory(category Category) {
// 	f.Categories = slices.DeleteFunc(f.Categories, func(c Category) bool { return c == category })
// }

func (f SubscriptionFilters) IsView(view View) bool {
	return f.View == view
}

func (f *SubscriptionFilters) SetView(view View) {
	f.View = view
}

func (f SubscriptionFilters) GetView() View {
	return f.View
}

func (f *SubscriptionFilters) SetPagination(pagination Pagination) {
	f.Pagination = &pagination
}

func NewArticleFilters() ArticleFilters {
	return ArticleFilters{
		SortBy:    SortByUnreadCount,
		SortOrder: SortOrderDesc,
		Count:     DefaultCount,
		View:      DefaultView,
	}
}

// FiltersValidation is a custom struct-level validation function for Filters.
// In this case, we validate that either a list of Feeds or Items has been
// provided, and fail validation if both have been provided.
func ArticleFiltersValidation(sl validator.StructLevel) {
	filters := sl.Current().Interface().(ArticleFilters)
	// Cannot have both subscription IDs and article IDs.
	if len(filters.Subscriptions) > 0 && len(filters.Articles) > 0 {
		sl.ReportError(filters.Subscriptions, "subscriptions", "Subscriptions", "subscriptionsorarticles", "")
		sl.ReportError(filters.Articles, "articles", "articles", "subscriptionsorarticles", "")
	}
	// Cannot have both a list of article IDs and a list of categories.
	if len(filters.Articles) > 0 && len(filters.Categories) > 0 {
		sl.ReportError(filters.Articles, "articles", "articles", "articlesorcategories", "")
		sl.ReportError(filters.Categories, "categories", "categories", "categoriesorids", "")
	}
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *ArticleFilters) Valid() (bool, error) {
	// Register custom struct-level validation function.
	validation.AddStructValidationFunc(ArticleFiltersValidation, ArticleFilters{})
	// Set required filters to valid values as necessary.
	f.SortBy = setValidSortBy(f.SortBy)
	f.SortOrder = setValidSortOrder(f.SortOrder)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	// Validate struct.
	return validation.ValidateStruct(f)
}

// Sort returns the Sort object for the Filters.
func (f ArticleFilters) GetSort() Sort {
	return Sort{
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
	}
}

// CountAsInt returns the count value (encoded as a string in the filters) as an int.
func (f ArticleFilters) GetCount() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return 10
	}
	return value
}

func (f ArticleFilters) ToQueryParams() url.Values {
	params := make(url.Values)

	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}

	if len(f.Articles) > 0 {
		params.Set(ParamArticles, strings.Join(f.Articles, ","))
	}

	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}

	if f.Pagination != nil {
		params.Set(ParamPagination, *f.Pagination)
	}

	params.Set(ParamSortBy, string(f.SortBy))
	params.Set(ParamSortOrder, string(f.SortOrder))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)

	return params
}

func (f ArticleFilters) IsSorted(sort Sort) bool {
	return f.SortBy == sort.SortBy && f.SortOrder == sort.SortOrder
}

func (f *ArticleFilters) SetSort(sort Sort) {
	f.SortBy = sort.SortBy
	f.SortOrder = sort.SortOrder
}

func (f ArticleFilters) HasCategory(category Category) bool {
	return slices.Contains(f.Categories, category)
}

// func (f *ArticleFilters) AddCategory(category Category) {
// 	f.Categories = append(f.Categories, category)
// 	f.Categories = slices.Compact(f.Categories)
// }

// func (f *ArticleFilters) RemoveCategory(category Category) {
// 	f.Categories = slices.DeleteFunc(f.Categories, func(c Category) bool { return c == category })
// }

func (f ArticleFilters) IsView(view View) bool {
	return f.View == view
}

func (f *ArticleFilters) SetView(view View) {
	f.View = view
}

func (f ArticleFilters) GetView() View {
	return f.View
}

func (f *ArticleFilters) SetPagination(pagination Pagination) {
	f.Pagination = &pagination
}
