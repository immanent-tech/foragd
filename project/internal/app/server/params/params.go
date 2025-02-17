// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package params

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/joshuar/go-feed-me/internal/models"
)

type ParamsOption func(url.Values) url.Values

// WithFeeds option replaces any existing FeedID filters with the given list.
func WithFeeds(ids ...models.FeedID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("feeds", strings.Join(ids, ","))
		return v
	}
}

// WithItems option replaces any existing ItemID filters with the given list.
func WithItems(ids ...models.ItemID) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("items", strings.Join(ids, ","))
		return v
	}
}

// WithItems option replaces any existing ItemID filters with the given list.
func WithCategories(categories ...models.Category) ParamsOption {
	return func(v url.Values) url.Values {
		if len(categories) > 0 {
			v.Set("categories", strings.Join(categories, ","))
		}

		return v
	}
}

func WithView(view models.View) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("view", string(view))
		return v
	}
}

func WithMark(mark models.Mark) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("mark", string(mark))
		return v
	}
}

func WithCount(count int) ParamsOption {
	return func(v url.Values) url.Values {
		v.Set("count", strconv.Itoa(count))
		return v
	}
}

// BuildURL creates a new URL string with params defined by the given options.
func BuildURL(path string, options ...ParamsOption) string {
	params := make(url.Values)

	for _, option := range options {
		params = option(params)
	}

	return path + "?" + params.Encode()
}
