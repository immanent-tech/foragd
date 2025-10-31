// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/session"
)

// Logout handles logout requests.
func Logout() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Delete the session cookie.
		err := session.Manager.Destroy(req.Context())
		if err != nil {
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
		http.Redirect(res, req, logoutURL.String(), http.StatusTemporaryRedirect)
	}
}
