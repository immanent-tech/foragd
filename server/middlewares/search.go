/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
)

// CanonicalizeSearchParams processes and stores the search params requested by a user.
func CanonicalizeSearchParams(session handlers.SessionManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var (
				search    *models.SearchRequest
				urlHeader string
			)

			switch {
			case htmx.IsHistoryRestoreRequest(req):
				// History restore.
				search = handlerSearchRestore(req, session)
				urlHeader = htmx.HeaderPushURL
			case req.Method == http.MethodGet:
				// Get request.
				var redirect bool
				search, redirect = handleSearchGet(req)
				if redirect {
					req.URL.RawQuery = search.Encode()
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
				urlHeader = htmx.HeaderPushURL
			case req.Method == http.MethodPost:
				// Post request.
				search = handleSearchPost(req, session)
				urlHeader = htmx.HeaderReplaceUrl
			}

			// Save values.
			ctx := handlers.SearchParamsToCtx(req.Context(), search)
			handlers.SearchParamsToSession(ctx, session, search)

			// Update URL.
			newURL := req.URL.Clone()
			newURL.RawQuery = search.Encode()
			res.Header().Set(urlHeader, newURL.String())

			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func handlerSearchRestore(req *http.Request, session handlers.SessionManager) *models.SearchRequest {
	search := handlers.SearchParamsFromSession(req.Context(), session)
	slogctx.Debug(req.Context(),
		"Restored search from session.",
		slog.String("filters", search.Encode()),
	)
	// Reset parameters.
	count := search.GetFrom()
	search.Count = &count
	search.From = 0
	search.SearchAfter = nil
	return search
}

func handleSearchGet(
	req *http.Request,
) (*models.SearchRequest, bool) {
	// Regular GET: parse from query string
	search := models.ParseSearchParams(req.URL.Query())
	if err := search.Sanitise(); err != nil {
		slogctx.Warn(req.Context(), "Search sanitization failed.", slog.Any("error", err))
	}

	// Reset parameters on non-htmx requests.
	if !htmx.IsHTMX(req) {
		count := search.GetFrom()
		search.Count = &count
		search.From = 0
		search.SearchAfter = nil
	}

	// Check canonicalization - redirect if query changes
	canonical := search.Encode()
	return search, req.URL.RawQuery != canonical
}

func handleSearchPost(req *http.Request, session SessionManager) *models.SearchRequest {
	log := slogctx.FromCtx(req.Context())

	search, err := forms.DecodeForm[*models.SearchRequest](req)

	if err != nil || search == nil {
		// Try to restore params from session.
		log.Warn("Unable to decode search params. Using search params from session.",
			slog.Any("error", err),
			slog.Any("search", search),
		)
		search = handlers.SearchParamsFromSession(req.Context(), session)
	}
	fmt.Fprintf(os.Stdout, "%d\n", search.From)
	return search
}
