// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

// Landing handles displaying the landing page of the site.
func Landing() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		templ.Handler(templates.LandingPage()).ServeHTTP(res, req)
	}
}
