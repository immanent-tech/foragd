// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/models"
)

// InternalServerError handles errors related to non-specific internal server functionality failures.
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) {
	slogctx.FromCtx(req.Context()).Error("Cannot display content.",
		slog.Any("error", models.NewMessage("Internal server error", models.MessageStatusError, models.WithError(err))))
	http.Error(res, "Problem!", http.StatusInternalServerError)
}
