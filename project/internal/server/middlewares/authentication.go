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

package middlewares

import (
	"fmt"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/platforms/postgres"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

// IsAuthenticated checks whether the session user is a valid user.
func IsAuthenticated(req *http.Request, db *postgres.Client) (bool, error) {
	// Get the user tokens from the session storage.
	tokens, err := session.GetTokens(req.Context())
	if err != nil {
		return false, fmt.Errorf("unable to get user details from session: %w", err)
	}
	// Ensure the user ID in the token matches a user in the database.
	_, err = db.GetUserByID(req.Context(), tokens.UserID())
	if err != nil {
		return false, fmt.Errorf("unable to get user details from session: %w", err)
	}

	return true, nil
}
