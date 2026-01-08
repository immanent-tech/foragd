// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/session"
)

// Logout handles logout requests.
func Logout(res http.ResponseWriter, req *http.Request) {
	// Delete the session cookie.
	if err := session.Clear(req.Context()); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	// Generate logout URL.
	logoutURL, err := auth0.GenerateLogoutURL(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	// Redirect user to logout URL.
	slogctx.FromCtx(req.Context()).Info("User logged out.")
	http.Redirect(res, req, logoutURL.String(), http.StatusTemporaryRedirect)
}
