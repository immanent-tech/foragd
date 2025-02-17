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
