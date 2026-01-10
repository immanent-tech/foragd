// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
)

func About() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "About")
		renderPage(templates.About()).ServeHTTP(res, req.WithContext(ctx))
	}
}
