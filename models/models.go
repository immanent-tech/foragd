// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package models contains objects and fields representing common schema within the application.
package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"slices"
	"time"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultHTTPRequestTimeout is the maximum time allowed for a background HTTP request to execute.
	DefaultHTTPRequestTimeout = 30 * time.Second
	// DefaultRequestRetries is the default number of retries for API requests.
	DefaultRequestRetries = 3
	// DefaultPaginationSize is the default number of docs to fetch when paginating through results from elasticsearch.
	DefaultPaginationSize = 5000
)

var ErrInvalidDateTimeFormat = errors.New("datetime is invalid")

var UnixEpoch = time.Unix(0, 0)

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

func (p *ObjectParams) Valid() error {
	if err := validation.Validate.Struct(p); err != nil {
		return err
	}
	return nil
}

func (p *ObjectParams) Sanitise() error {
	return nil
}

func (m *MarkObjectParams) Valid() error {
	if err := validation.Validate.Struct(m); err != nil {
		return err
	}
	return nil
}

func (m *MarkObjectParams) Sanitise() error {
	return nil
}

// validateDatetime will check whether a time.Time is not either the zero value or equal to the Unix epoch.
func validateDatetime(dt time.Time) (bool, error) {
	switch {
	case dt.IsZero():
		return false, fmt.Errorf("%w: is zero time value", ErrInvalidDateTimeFormat)
	case dt.Equal(UnixEpoch):
		return false, fmt.Errorf("%w: is unix epoch", ErrInvalidDateTimeFormat)
	default:
		return true, nil
	}
}

func (r *ViewComponent) Render(ctx context.Context, w io.Writer) error {
	if r.Component != nil {
		if err := r.Component.Render(ctx, w); err != nil {
			return fmt.Errorf("render template data component: %w", err)
		}
	}
	return nil
}
