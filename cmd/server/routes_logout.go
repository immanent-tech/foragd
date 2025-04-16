// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"
)

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}
