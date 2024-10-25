// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go run golang.org/x/tools/cmd/stringer -type=Option -linecomment -output id_generated.go
package id

import (
	"fmt"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	Sub  Option = iota // sub
	Feed               // feed
	Item               // item
)

// Option represents a type of ID. Specific types share a common prefix.
type Option int

// NewID generates a new unique ID for the given type option. If an ID cannot be
// generated, a non-nil error is returned.
func NewID(option Option) (string, error) {
	id, err := nanoid.Nanoid()
	if err != nil {
		return "", fmt.Errorf("could not generate username: %w", err)
	}

	return option.String() + "_" + id, nil
}
