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

	"github.com/joshuar/go-feed-me/internal/models"
)

func StorePreferences(ctx context.Context, preferences models.UserPreferences) {
	sessionManager.Put(ctx, preferencesSessionKey, preferences)
}

func GetPreferences(ctx context.Context) (models.UserPreferences, error) {
	data := sessionManager.Get(ctx, preferencesSessionKey)
	preferences, ok := data.(models.UserPreferences)

	switch {
	case data == nil:
		return nil, ErrDataNotFound
	case ok:
		return preferences, nil
	default:
		return nil, ErrInvalidData
	}
}
