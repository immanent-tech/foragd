// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/views"
)

// SignupSetup handles setting up a new user sign up request.
func SignupSetup() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).Then(RenderResponse(models.NewResponse(
		models.WithResponseTemplate(views.SignupForm(&models.UserSignupRequest{})),
	))).ServeHTTP
}

// Signup handles processing a user sign up request and showing the result.
func (a *API) Signup() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		var err error
		// Extract the provider and sign up request details.
		provider := chi.URLParam(req, "provider")
		newUser, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Create the new account on the provider backend.
		var (
			externalUserID string
			resp           *models.Response
		)
		switch provider {
		case "auth0":
			externalUserID, resp = a.UserAPI().Create(req.Context(), newUser)
		}
		if resp != nil {
			resp.Template = partials.Alert(partials.MsgBackendErr())
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
			return
		}
		// Create the local user account.
		user := models.NewUser(externalUserID, provider)
		valid, err = user.Valid(req.Context())
		if err != nil || !valid {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		index := elastic.UserIndexFromCtx(req.Context())
		if index == "" {
			chain.Then(RenderResponse(RespBackendError(elastic.ErrFetchCtx))).ServeHTTP(res, req)
			return
		}
		err = elastic.CreateDoc(req.Context(), a.DataAPI().GetAPI(), index, user.GetID(), user)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		chain.Then(RenderResponse(models.NewResponse(
			models.WithResponseTemplate(views.SignupSuccess()),
		))).ServeHTTP(res, req)
	}
}
