// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	feeds "github.com/immanent-tech/go-syndication"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func Viewer() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "Feed Viewer")
		switch req.Method {
		case http.MethodGet:
			renderPage(templates.Viewer()).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			// Get the submitted URL.
			url := req.Form.Get("url")

			// Parse the URL and find feed content.
			feed, err := feeds.NewFeedFromURL(req.Context(), url)
			if err != nil {
				renderPartial(templates.ErrorMessage(
					models.NewErrorMessage(
						"Unable inspect URL",
						"This might be a temporary issue, please try again",
					),
				)).ServeHTTP(res, req.WithContext(ctx))
				return
			}

			renderPartial(templates.ViewerResults(feed)).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}
