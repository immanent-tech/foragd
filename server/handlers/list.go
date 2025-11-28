// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
)

// WatchList handles watching a list of object for any updates and rendering a notification to the user to refresh the page.
func WatchList(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := api.BuildItemsQuery(req.Context(), filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch list for updates.
		watchForUpdates(api, query).ServeHTTP(res, req)
	}).ServeHTTP
}
