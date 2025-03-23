// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import "iter"

const (
	// FeedsRoute is the base path for showing a list of feeds.
	FeedsRoute = "/home/feeds"
	// ItemsRoute is the base path for showing a list of items.
	ItemsRoute = "/home/items"
)

// Option is a generic functional option that can be used by any type that needs
// to implement its own options.
type Option[T any] func(T)

// filterSlice is an iterator to filter slice values.
//
// https://www.dolthub.com/blog/2024-12-20-collection-functions-in-go-1-23/#the-missing-methods-slicesmap-and-slicesfilter
func filterSlice[S any](s []S, fn func(S) bool) iter.Seq[S] {
	return func(yield func(s S) bool) {
		for _, v := range s {
			if fn(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}
