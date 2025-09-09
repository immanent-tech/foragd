// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/justinas/alice"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/server/forms"
	"github.com/immanent-tech/go-feed-me/web/templates/pages"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
)

// ShowSignup handles showing a signup page.
func (a *API) ShowSignup() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).Then(
		renderPage(pages.Signup(&models.UserSignupRequest{}), "Sign up - Go Feed Me"),
	).ServeHTTP
}

// ProcessSignup handles processing a user sign up request.
func (a *API) ProcessSignup() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Process user signup.
		// Extract the provider and request details.
		request, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
		if err != nil || !valid {
			msg := models.NewWarningMessage(
				"Invalid signup details.",
				"Could not validate the values. Please check and try again.",
			)
			template := templ.Join(pages.SignupForm(request), partials.Notification(msg))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Create the new account on the provider backend.
		var externalUserID string
		externalUserID, err = a.UserAPI().Create(req.Context(), request)
		if err != nil {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			template := templ.Join(pages.SignupForm(request), partials.Notification(msg))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Create the local user account.
		user := models.NewUser(externalUserID, "auth0")
		valid, err = user.Valid(req.Context())
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			template := templ.Join(pages.SignupForm(request), partials.Notification(msg))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		index := elastic.UserIndexFromCtx(req.Context())
		if index == "" {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			template := templ.Join(pages.SignupForm(request), partials.Notification(msg))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = elastic.CreateDoc(req.Context(), a.DataAPI().GetAPI(), index, user.GetID(), user)
		if err != nil {
			msg := models.NewErrorMessage(
				"User creation failed.",
				"The backend had issues trying to create a new user, please try again.",
			)
			template := templ.Join(pages.SignupForm(request), partials.Notification(msg))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template := templ.Join(pages.SignupForm(request), pages.SignupSuccessNotification())
		renderPage(template, "").ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}
