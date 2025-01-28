// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

// String returns a URL-encoded string of query parameters. Only some query
// parameters are exposed this way.
func (f APIFilters) String() string {
	params := make(url.Values)

	if len(f.FeedIDs) > 0 {
		params.Add("feeds", strings.Join(f.FeedIDs, ","))
	}

	if len(f.ItemIDs) > 0 {
		params.Add("items", strings.Join(f.ItemIDs, ","))
	}

	if len(f.Categories) > 0 {
		params.Add("categories", strings.Join(f.Categories, ","))
	}

	// if f.Pagination != nil {
	// 	params.Add("pagination", string(f.Pagination))
	// }

	if f.ShowUnread != "" {
		params.Add("show_unread", "on")
	}

	if f.Count != 0 {
		params.Add("count", strconv.Itoa(f.Count))
	}

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

// CreateFilters takes an object, typically the params for a particular route,
// and generates a APIFilters object from them.
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
