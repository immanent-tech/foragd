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

	"github.com/elastic/go-elasticsearch/v8/typedapi"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
)

// RequireAuthentication ensures there is a valid user for the given protected
// routes.
func RequireAuthentication(protectedRoutes []string, api *typedapi.API) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if slices.ContainsFunc(protectedRoutes, func(path string) bool {
				return strings.HasPrefix(req.URL.Path, path)
			}) {
				// Fetch the user from the user management API.
				user, err := elastic.GetUser(elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix), api)
				//  If no user can be found, redirect back to the home page.
				if err != nil {
					logging.LogReq(req, http.StatusUnauthorized).
						Error("Authentication error.",
							slog.Any("error", err))
					http.Redirect(res, req, "/", http.StatusSeeOther)
				}
				// Else load the user into the context and pass the new context
				// to the next request.
				req = req.WithContext(models.UserToCtx(req.Context(), user))
			}

			next.ServeHTTP(res, req)
		})
	}
}
