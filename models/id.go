// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool golang.org/x/tools/cmd/stringer -type=IDPrefix -linecomment -output id.gen.go
package models

import (
	"strings"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	Invalid IDPrefix = iota + 0
	Min
	SubscriptionPFX // sub
	FeedPFX         // feed
	ItemPFX         // item
	SchedulerPFX    // scheduler
	SessionPFX      // session
	Max
)

// IDPrefix represents a type of ID. Specific types share a common prefix.
type IDPrefix int

// NewID generates a new unique ID for the given type option. If an ID cannot be
// generated, a non-nil error is returned.
func NewID(option IDPrefix) string {
	id, _ := nanoid.Nanoid()
	return option.String() + "_" + id
}

// IdentifyID takes an ID and returns the type of ID it represents.
func IdentifyID(id string) IDPrefix {
	idParts := strings.Split(id, "_")
	switch idParts[0] {
	case FeedPFX.String():
		return FeedPFX
	case ItemPFX.String():
		return ItemPFX
	case SubscriptionPFX.String():
		return SubscriptionPFX
	default:
		return Invalid
	}
}
