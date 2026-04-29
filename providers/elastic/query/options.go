// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package query

// Boostable represents a query option that supports a boost parameter.
type Boostable interface {
	*TermQuery | *TermsQuery | *ExistsQuery | *MatchAllQuery

	SetBoost(boost float32)
}

// WithQueryBoost applies a boost to this query option.
func WithQueryBoost[T Boostable](boost float32) func(T) {
	return func(v T) {
		v.SetBoost(boost)
	}
}

// Nameable represents a query option that can be named.
type Nameable interface {
	*TermQuery | *TermsQuery | *ExistsQuery | *MatchAllQuery

	SetName(name string)
}

// WithQueryName assigns the given name to this query option.
func WithQueryName[T Nameable](name string) func(T) {
	return func(v T) {
		v.SetName(name)
	}
}
