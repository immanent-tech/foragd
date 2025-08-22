// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/pages"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// ShowSignup handles showing a signup page.
func (a *API) ShowSignup() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Show form for user signup.
		template := pages.NewSignup(nil).Content()
		if !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req) {
			template = templates.Page("Sign up - Go Feed Me", template)
		}
		resp := models.NewResponse(models.WithResponseTemplate(template))
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// ProcessSignup handles processing a user sign up request.
func (a *API) ProcessSignup() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Process user signup.
		// Extract the provider and request details.
		newUser, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
		if err != nil || !valid {
			msg := models.NewWarningMessage(
				"Invalid signup details.",
				"Could not validate the values. Please check and try again.",
			)
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusUnprocessableEntity),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.Notification(msg))))).ServeHTTP(res, req)
			return
		}
		// Create the new account on the provider backend.
		var externalUserID string
		externalUserID, err = a.UserAPI().Create(req.Context(), newUser)
		if err != nil {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.ServerErrorNotification(msg))))).ServeHTTP(res, req)
			return
		}
		// Create the local user account.
		user := models.NewUser(externalUserID, "auth0")
		valid, err = user.Valid(req.Context())
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.ServerErrorNotification(msg))))).ServeHTTP(res, req)
			return
		}
		index := elastic.UserIndexFromCtx(req.Context())
		if index == "" {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.ServerErrorNotification(msg))))).ServeHTTP(res, req)
			return
		}
		err = elastic.CreateDoc(req.Context(), a.DataAPI().GetAPI(), index, user.GetID(), user)
		if err != nil {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.ServerErrorNotification(msg))))).ServeHTTP(res, req)
			return
		}
		msg := models.NewSuccessMessage(
			"Account created!",
			"",
		)
		resp := models.NewResponse(
			models.WithResponseTemplate(partials.Notification(msg)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}
