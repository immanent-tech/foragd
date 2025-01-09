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
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

// RequireAuthentication ensures there is a valid user for the given protected
// routes. For protected routes, it retrieves the user tokens from the session
// store, then validates if the token matches a valid user in the database
// store. For unprotected routes, it passes the request along unmodified.
func RequireAuthentication(protectedRoutes []string, userMgmtAPI models.UserManagementAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if slices.ContainsFunc(protectedRoutes, func(path string) bool {
				return strings.HasPrefix(req.URL.Path, path)
			}) {
				// Get the user tokens from the session storage.
				tokens, err := session.GetTokens(req.Context())
				if err != nil {
					logging.LogReq(req, http.StatusUnauthorized).Error("Authentication error.", slog.Any("error", err))
					http.Error(res, "Authentication error.", http.StatusUnauthorized)

					return
				}
				// Ensure the user ID in the token matches a user in the database.
				valid, err := userMgmtAPI.UserExists(req.Context(), tokens.UserID())
				if err != nil || !valid {
					logging.LogReq(req, http.StatusUnauthorized).Error("Authentication error.", slog.Any("error", err))
					http.Error(res, "Authentication error.", http.StatusUnauthorized)

					return
				}
			}

			next.ServeHTTP(res, req)
		})
	}
}
