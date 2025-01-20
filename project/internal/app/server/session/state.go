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

package session

import (
	"context"
)

func StoreState(ctx context.Context, state string) error {
	sessionManager.Put(ctx, stateSessionKey, state)

	return nil
}

func GetState(ctx context.Context) (string, error) {
	data := sessionManager.Get(ctx, stateSessionKey)
	preferences, ok := data.(string)

	switch {
	case data == nil:
		return "", ErrDataNotFound
	case ok:
		return preferences, nil
	default:
		return "", ErrInvalidData
	}
}
