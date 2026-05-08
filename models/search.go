// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/textquerytype"
	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

// NewSearchRequest creates a new SearchRequest object with default values. Defaults are search all objects within last
// week, sorted by most relevant.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		PublishedWithin: SearchRequestPublishedWithinLastWeek,
		View:            ViewAll,
		Sort:            SortNewestFirst,
	}
}

// Valid returns a boolean indicating whether the search request data is valid.
func (r *SearchRequest) Valid() error {
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
		r.Subscriptions[idx] = sanitization.SanitizeString(subscription)
	}
	// Sanitise text inputs.
	r.Text = sanitization.SanitizeString(r.Text)
	if r.Authors != nil {
		cleanAuthors := sanitization.SanitizeString(*r.Authors)
		r.Authors = &cleanAuthors
	}
	if r.Authors != nil {
		cleanCategories := sanitization.SanitizeString(*r.Categories)
		r.Categories = &cleanCategories
	}
	// Default timezone is UTC.
	if r.Timezone == "" {
		r.Timezone = "UTC"
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
		r.Sort = SortMostRelevant
	}
	return nil
}

// Query returns a string that represents the search as query parameters.
func (r *SearchRequest) Query() string {
	return r.params().Encode()
}

// HXVals returns a string that represents the search as hx-vals.
func (r *SearchRequest) HXVals() string {
	data, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(data)
}

func (r *SearchRequest) Values() map[string]any {
	params := make(map[string]any)
	params["text"] = r.Text
	if r.Authors != nil {
		params["authors"] = *r.Authors
	}
	if r.Categories != nil {
		params[ParamCategories] = *r.Categories
	}
	if len(r.Subscriptions) > 0 {
		params[ParamSubscriptions] = strings.Join(r.Subscriptions, ",")
	}
	params[ParamView] = string(r.View)
	params["published_within"] = string(r.PublishedWithin)
	params[ParamSort] = string(r.Sort)
	params["timezone"] = r.Timezone
	if r.SubscriptionID != nil {
		params[ParamSubscriptionID] = *r.SubscriptionID
	}
	return params
}

func (r *SearchRequest) params() url.Values {
	params := make(url.Values)
	params.Set("text", r.Text)
	if r.Authors != nil {
		params.Set("authors", *r.Authors)
	}
	if r.Categories != nil {
		params.Set(ParamCategories, *r.Categories)
	}
	if len(r.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(r.Subscriptions, ","))
	}
	params.Set(ParamView, string(r.View))
	params.Set("published_within", string(r.PublishedWithin))
	params.Set(ParamSort, string(r.Sort))
	params.Set("timezone", r.Timezone)
	if r.SubscriptionID != nil {
		params.Set(ParamSubscriptionID, *r.SubscriptionID)
	}
	return params
}

// Valid returns a boolean indicating whether the add subscription search filter data is valid.
func (r *AddSubscriptionSearchFilterRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription search filer is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the add subscription search filter request.
func (r *AddSubscriptionSearchFilterRequest) Sanitise() error {
	r.InputName = sanitization.SanitizeString(r.InputName)
	r.SubscriptionName = sanitization.SanitizeString(r.SubscriptionName)
	r.SubscriptionID = sanitization.SanitizeString(r.SubscriptionID)
	return nil
}

func SearchResultsClause(search *SearchRequest) query.BoolOption {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return query.Must(
		// Search across title, description and content fields, with preference for match in that order (via field
		// boosting).
		query.Bool(
			query.Should(
				query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
				query.SimpleQueryString(
					query.WithSimpleQueryStringText(&search.Text),
					query.WithSimpleQueryStringFields("title^6", "description^3", "content"),
					query.WithSimpleQueryStringOperator(&operator.And),
				),
				query.MultiMatch(
					search.Text,
					[]string{"description^3", "content"},
					query.WithTextQueryType(textquerytype.Phrase),
				),
			),
		),
		// Search in categories.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		// Search in authors, contributors.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

func SearchSuggestionsClause(search *SearchRequest) query.BoolOption {
	// Must match at least one of in title, description, content.
	return query.Must(
		query.Bool(
			query.Should(
				query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
				query.SearchAsYouType(search.Text, "title"),
				query.SearchAsYouType(search.Text, "description"),
				query.SimpleQueryString(
					query.WithSimpleQueryStringText(&search.Text),
					query.WithSimpleQueryStringFields("content"),
					query.WithSimpleQueryStringOperator(&operator.And),
				),
			),
		),
	)

}
