// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
