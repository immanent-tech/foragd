// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package models contains objects and fields representing common schema within the application.
package models

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"time"
)

type Option[T any] func(T)

var (
	ErrInvalidID             = errors.New("error generating unique ID")
	ErrInvalidDateTimeFormat = errors.New("datetime is invalid")
)

var UnixEpoch = time.Unix(0, 0)

type UserData struct {
	*Tokens
	*User
}

func SliceToMap[K comparable, V any, S any](s []S, mapFn func(S) (K, V)) map[K]V {
	m := make(map[K]V)
	for _, k := range s {
		key, val := mapFn(k)
		m[key] = val
	}
	return m
}

// FilterSlice will filter a slice by the given function.
func FilterSlice[S any](s []S, fn func(S) bool) iter.Seq[S] {
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
