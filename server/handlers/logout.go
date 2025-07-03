// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/markbates/goth/gothic"
	slogctx "github.com/veqryn/slog-context"
)

// Logout handles logout requests.
func Logout() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slogctx.FromCtx(req.Context()).Info("logging out")
		if err := gothic.Logout(res, req); err != nil {
			slogctx.FromCtx(req.Context()).Error("Logout failed.",
				slog.Any("error", err))
		}
		slogctx.FromCtx(req.Context()).Debug("User logged out.")
		res.Header().Set("Location", "/")
		res.WriteHeader(http.StatusTemporaryRedirect)
	}
}
