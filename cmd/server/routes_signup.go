// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/signup"
)

// SignUp handles presenting a form for the user to enter sign-up details.
func (s Server) SignUp(res http.ResponseWriter, req *http.Request) {
	resp := htmx.NewResponse()

	fullPage := layouts.BuildPage(
		layouts.WithHeadOptions("Sign-up to Go Feed Me",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		),
		layouts.WithPageContent(signup.Show(models.NewUserSignup())),
	)
	if err := resp.RenderTempl(req.Context(), res, fullPage.Show()); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
}

// ProcessSignUp handles validating and processing a user sign-up request.
func (s Server) ProcessSignUp(res http.ResponseWriter, req *http.Request) {
	resp := htmx.NewResponse()
	// Decode and validate the user sign-up request.
	userSignup, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
	if err != nil || !valid {
		if err := resp.RenderTempl(req.Context(), res, signup.SignupForm(userSignup)); err != nil {
			logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
			http.Error(res, "user signup failed!", http.StatusInternalServerError)
		}
		return
	}
	// Process the user sign-up and create the new user.
	if err := s.addUser(req.Context(), userSignup); err != nil {
		userSignup.Msg = backendErrorMsg(err)
		if err := resp.RenderTempl(req.Context(), res, signup.SignupForm(userSignup)); err != nil {
			logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
			http.Error(res, "user signup failed!", http.StatusInternalServerError)
		}
		return
	}
	// Display success and prompt user to log in with new account.
	if err := resp.Retarget(signup.SignupDetailsID.Target()).Reswap(htmx.SwapOuterHTML).RenderTempl(req.Context(), res, signup.SignupSuccess()); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
		http.Error(res, "user signup failed!", http.StatusInternalServerError)
	}
}

func (s Server) addUser(ctx context.Context, userSignup *models.UserSignupRequest) error {
	// Create the user in the auth backend.
	userID, err := s.API.user.Create(ctx, userSignup)
	if err != nil {
		return models.WrapError(err, "routes/signup", "could not create user in auth backend")
	}

	// Create new user in the database backend.
	addUserCtx := elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)

	err = s.DataAPI().AddUser(addUserCtx, userID)
	if err != nil {
		return models.WrapError(err, "routes/signup", "could not create user in database backend")
	}

	return nil
}
