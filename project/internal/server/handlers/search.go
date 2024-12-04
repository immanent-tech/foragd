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
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

func Search(res http.ResponseWriter, req *http.Request) {
	filters, problems, err := DecodeRequest[*models.Filters](req)

	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode filters.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	if len(problems) > 0 {
		logging.FromContext(req.Context()).
			Debug("Problems were encountered when decoding filters.", slog.Any("problems", problems))
	}

	url, triggered := htmx.GetCurrentURL(req)
	spew.Dump(url, triggered, filters)

	res.WriteHeader(http.StatusNotImplemented)
	// searchRequest, problems, err := decodeForm[*models.SearchRequest](req)
	// if err != nil && len(problems) == 0 { // Internal validation error.
	// 	res.WriteHeader(http.StatusInternalServerError)
	// 	logging.LogReq(req, http.StatusInternalServerError).Error("Invalid search request.", slog.Any("error", err))

	// 	return
	// }

	// // if len(problems) > 0 { // Validation errors.
	// // 	showSignupError(req, res, problems)
	// // }

	// results := search.GenerateSearchResults(nil, searchRequest.Terms)

	// if req.FormValue("Terms") != "" {
	// 	if err := htmx.NewResponse().
	// 		RenderTempl(req.Context(), res, search.SearchResultsTempl(results)); err != nil {
	// 		logging.LogReq(req, http.StatusInternalServerError).Error("Cannot render search results.", slog.Any("error", err))
	// 		res.WriteHeader(http.StatusInternalServerError)

	// 		return
	// 	}
	// }
}
