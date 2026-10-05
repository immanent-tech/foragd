// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
	"hash/fnv"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/immanent-tech/foragd/models"
)

const (
	fragmentsCtxKey contextKey = "fragments"
)

type contextKey string

const (
	NavHome          Navigation = "home"
	NavSubscriptions Navigation = "subscriptions"
	NavArticles      Navigation = "articles"
	NavMap           Navigation = "map"
	NavFavorites     Navigation = "favorites"
	NavDiscover      Navigation = "discover"
	NavSettings      Navigation = "settings"
)

// Navigation reflects what navigation marker the current page should be placed under.
type Navigation string

const (
	routeSubscriptionsList     Route = "/list/subscriptions"
	routeSubscriptionsPaginate Route = "/subscriptions/paginate"
	routeSubscriptionsUpdates  Route = "/subscriptions/updates"
	routeArticlesList          Route = "/list/articles"
	routeArticlesPaginate      Route = "/articles/paginate"
	routeArticlesUpdates       Route = "/articles/updates"
)

// Route represents a route from which a page is served.
type Route string

type AppConfig interface {
	GetAppID() string
	GetAppVersion() string
	GetAppName() string
	IsProduction() bool
	GetBaseURL() *url.URL
}

type Breadcrumbs interface {
	Previous(ctx context.Context) (*url.URL, bool)
}

type SessionManager interface {
	Get(ctx context.Context, key string) any
}

func FragmentKeysToCtx(ctx context.Context, keys ...templFragmentKey) context.Context {
	if len(keys) > 0 {
		return context.WithValue(ctx, fragmentsCtxKey, keys)
	}
	return ctx
}

func FragmentKeysFromCtx(ctx context.Context) []templFragmentKey {
	keys, found := ctx.Value(fragmentsCtxKey).([]templFragmentKey)
	if !found {
		return nil
	}
	return keys
}

// / initialLetter is the letter shown on the placeholder thumbnail.
func initialLetter(text string) string {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "#"
}

// hueFromText generates a hue (0-359) from the given text. Using the same text should generate a stable hue color.
func hueFromText(text string) int {
	h := fnv.New32a()
	h.Write([]byte(text))
	return int(h.Sum32() % 360)
}

// authorNames generates a comma-separated list of author names from the slice of [models.Author].
func authorNames(authors []models.Author) string {
	names := make([]string, 0, len(authors))
	for author := range slices.Values(authors) {
		if author.Name != "" {
			names = append(names, author.Name)
		}
	}
	return strings.Join(names, ",")
}

func articleURL(id string) string { return "/articles/" + id }
