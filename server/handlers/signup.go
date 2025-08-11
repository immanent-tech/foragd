// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/pages"
)

// Signup handles processing a user sign up request and showing the result.
func (a *API) Signup() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		switch req.Method {
		case http.MethodGet:
			// Show form for user signup.
			resp := models.NewResponse(
				models.WithResponseTemplate(pages.NewSignup(nil, nil).Template(req)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		case http.MethodPost:
			// Process user signup.
			// Extract the provider and request details.
			newUser, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
			if err != nil || !valid {
				resp := models.NewResponse(
					models.WithResponseStatusCode(http.StatusUnprocessableEntity),
					models.WithResponseError(err),
					models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Signup request is invalid.",
						Details: "Could not validate the values, please check and try again.",
					}).Template(req)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			// Create the new account on the provider backend.
			var externalUserID string
			externalUserID, err = a.UserAPI().Create(req.Context(), newUser)
			if err != nil {
				resp := models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Failed to create new user.",
						Details: "The backend had an issue trying to complete the request. Please try again.",
					}).Template(req)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			// Create the local user account.
			user := models.NewUser(externalUserID, "auth0")
			valid, err = user.Valid(req.Context())
			if err != nil || !valid {
				resp := models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Failed to create new user.",
						Details: "The backend had an issue trying to complete the request. Please try again.",
					}).Template(req)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			index := elastic.UserIndexFromCtx(req.Context())
			if index == "" {
				resp := models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Failed to create new user.",
						Details: "The backend had an issue trying to complete the request. Please try again.",
					}).Template(req)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			err = elastic.CreateDoc(req.Context(), a.DataAPI().GetAPI(), index, user.GetID(), user)
			if err != nil {
				resp := models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Failed to create new user.",
						Details: "The backend had an issue trying to complete the request. Please try again.",
					}).Template(req)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			resp := models.NewResponse(
				models.WithResponseTemplate(pages.NewSignup(newUser, &models.UserMessage{
					Status:  models.UserMessageStatusSuccess,
					Summary: "Account created!",
				}).Template(req)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		}
	}
}
