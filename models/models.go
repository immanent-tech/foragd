// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package models contains objects and fields representing common schema within the application.
package models

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"net/url"
	"slices"
	"time"
)

var ErrInvalidDateTimeFormat = errors.New("datetime is invalid")

var UnixEpoch = time.Unix(0, 0)

const (
	// DefaultMaxHistory for users/objects is 30 days.
	DefaultMaxHistory = 30 * 24 * time.Hour
)

// parseMaxHistory will parse the maxHistory string as a time.Duration, subtract it from the current time and return the
// time.Time value.
func parseMaxHistory(maxHistory string) time.Time {
	if maxHistory == "" {
		return time.Now().Add(-DefaultMaxHistory)
	}

	dur, err := time.ParseDuration(maxHistory)
	if err != nil {
		return time.Now().Add(-DefaultMaxHistory)
	}

	return time.Now().Add(-dur)
}

// SliceToMap generates a map from slice content by mapping key-value pairs from the slice with the given map function.
func SliceToMap[K comparable, V any, S any](s []S, mapFn func(S) (K, V)) map[K]V {
	m := make(map[K]V)
	for _, k := range s {
		key, val := mapFn(k)
		m[key] = val
	}
	return m
}

// FilterSlice will filter a slice returning an iter with elements that return true for the given filter function.
func FilterSlice[E any](s []E, fn func(E) bool) iter.Seq[E] {
	return func(yield func(s E) bool) {
		for _, v := range s {
			if fn(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// FilterMap will filter a map by the given function.
func FilterMap[K comparable, V any](m map[K]V, fn func(K, V) bool) iter.Seq2[K, V] {
	return func(yield func(k K, v V) bool) {
		for k, v := range m {
			if fn(k, v) {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// FilterMapValues will filter map values by the given function.
func FilterMapValues[K comparable, V any](s map[K]V, fn func(V) bool) iter.Seq[V] {
	return FilterSlice(slices.Collect(maps.Values(s)), fn)
}

// ValidateDatetime will check whether a time.Time is not either the zero value or equal to the Unix epoch.
func ValidateDatetime(dt time.Time) (bool, error) {
	switch {
	case dt.IsZero():
		return false, fmt.Errorf("%w: is zero time value", ErrInvalidDateTimeFormat)
	case dt.Equal(UnixEpoch):
		return false, fmt.Errorf("%w: is unix epoch", ErrInvalidDateTimeFormat)
	default:
		return true, nil
	}
}

func ParseDomain(value string) string {
	url, err := url.Parse(value)
	if err != nil {
		return value
	}
	return url.Hostname()
}

type List[T any] struct {
	Head, Tail *Element[T]
}

type Element[T any] struct {
	Next *Element[T]
	Val  T
}

func (lst *List[T]) Push(v T) {
	if lst.Tail == nil {
		lst.Head = &Element[T]{Val: v}
		lst.Tail = lst.Head
	} else {
		lst.Tail.Next = &Element[T]{Val: v}
		lst.Tail = lst.Tail.Next
	}
}

func (lst *List[T]) AllElements() []T {
	var elems []T
	for e := lst.Head; e != nil; e = e.Next {
		elems = append(elems, e.Val)
	}
	return elems
}

type HasFeedInfo interface {
	GetFeedID() FeedID
}

func GetFeedIDs[T HasFeedInfo](objects iter.Seq[T]) []FeedID {
	var ids []FeedID
	for details := range objects {
		ids = append(ids, details.GetFeedID())
	}
	return ids
}

func FindByFeedID[T HasFeedInfo](id FeedID, objects iter.Seq[T]) (T, bool) {
	for object := range objects {
		if object.GetFeedID() == id {
			return object, true
		}
	}
	return *new(T), false
}

type HasCategories interface {
	GetCategories() []Category
}

func GetCategories[T HasCategories](objects iter.Seq[T]) []Category {
	var categories []string
	for object := range objects {
		categories = append(categories, object.GetCategories()...)
	}
	slices.Sort(categories)
	return slices.Compact(categories)
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func GetCategoryCounts[T HasCategories](objects iter.Seq[T]) CategoryCounts {
	countsMap := make(map[Category]int)
	for object := range objects {
		for category := range slices.Values(object.GetCategories()) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

func (c ObjectCustomisation) GetNickname() string {
	return c.Nickname
}

func (c ObjectCustomisation) GetCategories() []Category {
	return c.Categories
}
