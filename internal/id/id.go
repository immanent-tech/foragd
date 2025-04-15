// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool golang.org/x/tools/cmd/stringer -type=Prefix -linecomment -output id_generated.go
package id

import (
	"strings"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	Invalid Prefix = iota + 0
	Min
	Subscription // sub
	Feed         // feed
	Item         // item
	Scheduler    // scheduler
	Session      // session
	Max
)

// Prefix represents a type of ID. Specific types share a common prefix.
type Prefix int

// Valid returns a boolean indicating whether the prefix is valid.
func (p Prefix) Valid() bool {
	return p > Min && p < Max
}

// NewID generates a new unique ID for the given type option. If an ID cannot be
// generated, a non-nil error is returned.
//
//nolint:errcheck
func NewID(option Prefix) string {
	id, _ := nanoid.Nanoid()
	return option.String() + "_" + id
}

// IdentifyID takes an ID and returns the type of ID it represents.
func IdentifyID(id string) Prefix {
	idParts := strings.Split(id, "_")
	switch idParts[0] {
	case Feed.String():
		return Feed
	case Item.String():
		return Item
	case Subscription.String():
		return Subscription
	default:
		return Invalid
	}
}
