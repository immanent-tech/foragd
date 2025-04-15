// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/markbates/goth/gothic"

	"github.com/joshuar/go-feed-me/internal/models"
)

// ProcessImportMethod will parse which import method has been chosen from the request, then call the appropriate
// handler for handling that type of import.
func AuthCallback(api AuthAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user, err := api.CompleteUserAuth(res, req)
		if err != nil {
			fmt.Fprintln(res, err)
			return
		}
		spew.Dump(user)
	})
}

func Login(api AuthAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if gothUser, err := api.CompleteUserAuth(res, req); err == nil {
			spew.Dump(gothUser)
			// Redirect to logged in page.
			req.Header.Add("Content-Type", "")
			http.Redirect(res, req, models.FeedsRoute, http.StatusTemporaryRedirect)
		} else {
			BeginAuthHandler(api).ServeHTTP(res, req)
		}
	})
}

func Logout() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		gothic.Logout(res, req)
		res.Header().Set("Location", "/")
		res.WriteHeader(http.StatusTemporaryRedirect)
	})
}

func BeginAuthHandler(api AuthAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		url, err := api.GetAuthURL(req)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(res, err)
			return
		}

		http.Redirect(res, req, url, http.StatusTemporaryRedirect)
	})
}
