// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/providers/auth0"
	"github.com/immanent-tech/go-feed-me/server/session"
)

// Logout handles logout requests.
func Logout() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Delete the session cookie.
		err := session.Manager.Destroy(req.Context())
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Logout failed.",
				slog.Any("error", err))
		}
		// Generate logout URL.
		logoutURL, err := auth0.GenerateLogoutURL(req)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Logout failed.",
				slog.Any("error", err))
		}
		// Redirect user to logout URL.
		http.Redirect(res, req, logoutURL, http.StatusTemporaryRedirect)
	}
}
