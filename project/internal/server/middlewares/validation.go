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
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/logging"
)

var (
	ErrPathMismatch  = errors.New("request path does not match handler path")
	ErrNotHTMX       = errors.New("request is not generated via htmx")
	ErrInvalidMethod = errors.New("invalid method")
)

func ValidateRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		routePattern := chi.RouteContext(req.Context()).RoutePattern()
		routeMethod := chi.RouteContext(req.Context()).RouteMethod
		if req.URL.Path != routePattern {
			logging.LogReq(req, http.StatusNotFound).
				Error("Route does not exist.")
			http.NotFound(res, req)
			return
		}
		if req.Method != routeMethod {
			logging.LogReq(req, http.StatusNotFound).
				Error("Method is not valid.")
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}
