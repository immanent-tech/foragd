// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

// InternalServerError handles errors related to non-specific internal server functionality failures.
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) {
	err = models.WrapError(err, req.URL.Path, "internal server error")
	logging.FromContext(req.Context()).Error("Cannot display content.",
		slog.Any("error", err))
	http.Error(res, "Problem!", http.StatusInternalServerError)
}

func HandleHTMXResponse(resp htmx.Response, templates ...templ.Component) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			for template := range slices.Values(templates) {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					logging.FromContext(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
				}
			}
		})
}
