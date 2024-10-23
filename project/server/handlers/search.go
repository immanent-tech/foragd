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

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/model"
	"github.com/joshuar/go-feed-me/templates/search"
)

func Search(res http.ResponseWriter, req *http.Request) {
	searchRequest, problems, err := decodeForm[*model.SearchRequest](req)
	if err != nil && len(problems) == 0 { // Internal validation error.
		res.WriteHeader(http.StatusInternalServerError)
		logging.LogReq(req, http.StatusInternalServerError).Error("Invalid search request.", slog.Any("error", err))

		return
	}

	// if len(problems) > 0 { // Validation errors.
	// 	showSignupError(req, res, problems)
	// }

	results := search.GenerateSearchResults(nil, searchRequest.Terms)

	if req.FormValue("Terms") != "" {
		if err := htmx.NewResponse().
			RenderTempl(req.Context(), res, search.SearchResultsTempl(results)); err != nil {
			logging.LogReq(req, http.StatusInternalServerError).Error("Cannot render search results.", slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)

			return
		}
	}
}
