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

package elastic

import (
	"errors"
)

var ErrNoClient = errors.New("no client")

// Option is a generic type for request options.
type Option[T any] func(T) T

type customisableIndexPattern[T any] interface {
	Index(value string) T
}

// WithValue allows setting an value on a component.
func WithIndexPattern[T any](value string) Option[T] {
	return func(req T) T {
		if settable, ok := any(req).(customisableIndexPattern[T]); ok {
			req = settable.Index(value)
		}

		return req
	}
}
