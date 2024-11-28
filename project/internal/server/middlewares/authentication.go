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
)

func IsAuthenticated(req *http.Request, pgMgr *postgres.Client) (bool, error) {
	// Ensure there are valid tokens in the session.
	user, err := pgMgr.GetUser(req.Context())
	if err != nil || user == nil {
		return false, fmt.Errorf("user is invalid: %w", err)
	}

	return true, nil
}
