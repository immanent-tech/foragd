// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package query

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/textquerytype"
)

// Boostable represents a query option that supports a boost parameter.
type Boostable interface {
	*TermQuery | *TermsQuery | *ExistsQuery | *MatchAllQuery | *MatchQuery | *MatchPhraseQuery | *MultiMatchQuery | *WildcardQuery

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
	*TermQuery | *TermsQuery | *ExistsQuery | *MatchAllQuery | *MatchQuery | *MatchPhraseQuery | *MultiMatchQuery | *WildcardQuery

	SetName(name string)
}

// WithQueryName assigns the given name to this query option.
func WithQueryName[T Nameable](name string) func(T) {
	return func(v T) {
		v.SetName(name)
	}
}

type FuzzinessValue types.Fuzziness

// Fuzziness represents a query option that supports setting fuzziness.
type Fuzziness interface {
	*MatchQuery | *MultiMatchQuery

	SetFuzziness(fuzziness FuzzinessValue)
	SetFuzzyTranspositions(value bool)
}

// WithFuzziness applies the given fuzziness to this query option.
func WithFuzziness[T Fuzziness](fuzziness FuzzinessValue) func(T) {
	return func(v T) {
		v.SetFuzziness(fuzziness)
	}
}

// WithFuzzyTranspositions allows edits for fuzzy matching include transpositions of two adjacent characters (ab → ba).
// Defaults to true if not set.
func WithFuzzyTranspositions[T Fuzziness](value bool) func(T) {
	return func(v T) {
		v.SetFuzzyTranspositions(value)
	}
}

// Sloppiness represents a query that can define a slop value.
type Sloppiness interface {
	*MatchPhraseQuery | *MultiMatchQuery

	SetSlop(slop int)
}

// WithSlop defines the maximum number of positions allowed between matching tokens.
func WithSlop[T Sloppiness](slop int) func(T) {
	return func(v T) {
		v.SetSlop(slop)
	}
}

type TextQueryType = textquerytype.TextQueryType

// TextQueryTypes represents a query that has different text query operating modes.
type TextQueryTypes interface {
	*MultiMatchQuery

	SetTextQueryType(tq TextQueryType)
}

// WithTextQueryType changes the operating mode of the text query.
func WithTextQueryType[T TextQueryTypes](tq TextQueryType) func(T) {
	return func(v T) {
		v.SetTextQueryType(tq)
	}
}
