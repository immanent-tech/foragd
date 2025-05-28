// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// InternalServerError handles errors related to non-specific internal server functionality failures.
func ResponseError(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.String("error", resp.String()))
	// Display a notification if a user message is set.
	if resp.UserMessage != nil {
		if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.ShowNotification(resp.UserMessage)); err != nil {
			http.Error(res, "Internal server error.", http.StatusInternalServerError)
		}
	}
	// Write the status code.
	res.WriteHeader(resp.StatusCode)
}
