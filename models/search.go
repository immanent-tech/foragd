// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/immanent-tech/go-base/validation"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/server/session"
)

const (
	// DefaultSearchSort is by most relevant.
	DefaultSearchSort = SortMostRelevant
	// defaultSearchTimezone is UTC.
	defaultSearchTimezone = "UTC"
	// DefaultSearchCount is the default number of search results to fetch at once.
	DefaultSearchCount = 15
)

// ParseSearchParams creates a new SearchRequest from the given URL query values.
func ParseSearchParams(query url.Values) *SearchRequest {
	search := NewSearchRequest()
	if v := query.Get("authors"); v != "" {
		search.Authors = &v
	}
	if v := query.Get("categories"); v != "" {
		search.Categories = &v
	}
	if v, err := strconv.Atoi(query.Get("from")); err == nil && v > 0 {
		search.From = &v
	}
	if v, err := strconv.Atoi(query.Get("upto")); err == nil && v > 0 {
		search.UpTo = &v
	}
	if a := query.Get("search_after"); a != "" {
		if pagination, err := url.QueryUnescape(a); err != nil {
			search.SearchAfter = &pagination
		}
	}
	if v := query.Get("published_within"); v != "" {
		search.PublishedWithin = SearchRequestPublishedWithin(v)
	}
	if v := query.Get("sort"); validSort[Sort(v)] {
		search.Sort = Sort(v)
	}
	if v := query.Get("view"); validView[View(v)] {
		search.View = View(v)
	}
	if v := query.Get("subscription_id"); v != "" {
		search.SubscriptionID = &v
	}
	if s := query.Get("subscriptions"); s != "" {
		search.Subscriptions = strings.Split(s, ",")
	}
	if v := query.Get("text"); v != "" {
		search.Text = v
	}
	if v := query.Get("timezone"); v != "" {
		search.Timezone = v
	}
	return search
}

// NewSearchRequest creates a new SearchRequest object with default values. Defaults are search all objects within last
// week, sorted by most relevant.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		PublishedWithin: SearchRequestPublishedWithinLastWeek,
		View:            ViewAll,
		Sort:            DefaultSearchSort,
		Timezone:        defaultSearchTimezone,
		Count:           DefaultSearchCount,
	}
}

// Valid returns a boolean indicating whether the search request data is valid.
func (r *SearchRequest) Validate() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("search request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the search request data.
func (r *SearchRequest) Sanitise() error {
	if r == nil {
		return nil
	}
	// Split and sanitise subscriptions field.
	if len(r.Subscriptions) == 1 {
		r.Subscriptions = strings.Split(r.Subscriptions[0], ",")
	}
	for idx, subscription := range r.Subscriptions {
		r.Subscriptions[idx] = validation.SanitizeString(subscription)
	}
	// Sanitise text inputs.
	r.Text = validation.SanitizeString(r.Text)
	if r.Authors != nil {
		cleanAuthors := validation.SanitizeString(*r.Authors)
		r.Authors = &cleanAuthors
	}
	if r.Authors != nil {
		cleanCategories := validation.SanitizeString(*r.Categories)
		r.Categories = &cleanCategories
	}
	// Default timezone is UTC.
	if r.Timezone == "" {
		r.Timezone = defaultSearchTimezone
	}
	// Default published within is last week.
	if r.PublishedWithin == "" {
		r.PublishedWithin = SearchRequestPublishedWithinLastWeek
	}
	// Default view is unread.
	if r.View == "" {
		r.View = ViewUnread
	}
	// Default sort is most relevant.
	if r.Sort == "" {
		r.Sort = DefaultSearchSort
	}
	return nil
}

// Encode returns a string that represents the search as query parameters.
func (r *SearchRequest) Encode() string {
	params := make(url.Values)
	params.Set("text", r.Text)
	if r.Authors != nil {
		params.Set("authors", *r.Authors)
	}
	if r.Categories != nil {
		params.Set("categories", *r.Categories)
	}
	if len(r.Subscriptions) > 0 {
		params.Set("subscriptions", strings.Join(r.Subscriptions, ","))
	}
	params.Set("view", string(r.View))
	params.Set("published_within", string(r.PublishedWithin))
	params.Set("sort", string(r.Sort))
	params.Set("count", strconv.Itoa(r.Count))
	params.Set("timezone", r.Timezone)
	if r.SubscriptionID != nil {
		params.Set("subscription_id", *r.SubscriptionID)
	}
	return params.Encode()
}

const searchParamsCtxKey contextKey = "searchParams"
const searchCountCtxKey contextKey = "searchCount"

// SearchParamsToCtx stores the given search params in the context.
func SearchParamsToCtx(ctx context.Context, search *SearchRequest) context.Context {
	return context.WithValue(ctx, searchParamsCtxKey, *search)
}

// SearchParamsFromCtx retrieves the given search params in the context.
func SearchParamsFromCtx(ctx context.Context) *SearchRequest {
	if search, ok := ctx.Value(searchParamsCtxKey).(SearchRequest); ok {
		return &search
	}
	slogctx.Warn(ctx, "No search params in context. Returning new search params.")
	return NewSearchRequest()
}

// SearchParamsToSession stores the given search params in the session.
func SearchParamsToSession(ctx context.Context, search *SearchRequest) {
	if err := session.Save(ctx, string(searchParamsCtxKey), *search); err != nil {
		slogctx.Warn(ctx, "Unable to save search params to session.", slog.Any("error", err))
	}
}

// SearchParamsFromSession retrieves the given search params in the session.
func SearchParamsFromSession(ctx context.Context) *SearchRequest {
	search, err := session.Restore[SearchRequest](ctx, string(searchParamsCtxKey))
	if err != nil {
		slogctx.Warn(ctx, "Unable to restore search params from session. Using defaults.", slog.Any("error", err))
		return NewSearchRequest()
	}
	return &search
}

// SearchCountToSession stores the current count of objects displayed in the search in the session.
func SearchCountToSession(ctx context.Context, count int) {
	if err := session.Save(ctx, string(searchCountCtxKey), count); err != nil {
		slogctx.Warn(ctx, "Unable to save search count to session.", slog.Any("error", err))
	}
}

// SearchCountFromSession stores the current count of objects displayed in the search in the session.
func SearchCountFromSession(ctx context.Context) int {
	count, err := session.Restore[int](ctx, string(searchCountCtxKey))
	if err != nil {
		slogctx.Warn(ctx, "Unable to restore list search count from session. Using defaults.", slog.Any("error", err))
		return defaultCount
	}
	return count
}

// Valid returns a boolean indicating whether the add subscription search filter data is valid.
func (r *AddSubscriptionSearchFilterRequest) Validate() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription search filer is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the add subscription search filter request.
func (r *AddSubscriptionSearchFilterRequest) Sanitise() error {
	r.InputName = validation.SanitizeString(r.InputName)
	r.SubscriptionName = validation.SanitizeString(r.SubscriptionName)
	r.SubscriptionID = validation.SanitizeString(r.SubscriptionID)
	return nil
}
