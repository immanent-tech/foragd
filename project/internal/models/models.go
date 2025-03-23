// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"iter"

	"github.com/mmcdole/gofeed"
)

type Option[T any] func(T)

var ErrInvalidID = errors.New("error generating unique ID")

// Feed represents a feed. It embeds the gofeed.Feed object and adds additional
// fields required.
type Feed struct {
	CreatedAt CreatedAt `json:"created_at" validate:"required"`
	*gofeed.Feed
	ID FeedID `json:"feed_id" validate:"required"`
}

// Item represents an item of a feed. It embeds the gofeed.Item object and adds additional
// fields required.
type Item struct {
	CreatedAt CreatedAt `json:"@timestamp" validate:"required"`
	*gofeed.Item
	ID     ItemID `json:"item_id"`
	FeedID FeedID `json:"feed_id"`
}

type UserData struct {
	*Tokens
	*User
}

func sliceToMap2[K comparable, V any, S any](s []S, mapFn func(S) (K, V)) map[K]V {
	m := make(map[K]V)
	for _, k := range s {
		key, val := mapFn(k)
		m[key] = val
	}
	return m
}

func filtered[S any](s []S, fn func(S) bool) iter.Seq[S] {
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
