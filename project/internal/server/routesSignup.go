// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

// SignUp serves the user sign up page.
// GET(/signup).
func (s Server) Signup(res http.ResponseWriter, req *http.Request) {
	var page templ.Component

	logger := s.Logger.With(slog.String("handler", "Signup"))

	ctx := layouts.UserSignupToCtx(req.Context(), models.NewUserSignup())

	page = layouts.Page("Go Feed Me - Sign-up",
		layouts.WithPageDescription("Sign-up to Go Feed Me."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.SignupLayout()))

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

// PostSignup processes the user sign up request.
// POST(/signup).
func (s Server) ProcessSignup(res http.ResponseWriter, req *http.Request) {
	logger := logging.NewHandlerLogger("ProcessSignup", req)

	userSignup, err := forms.DecodeForm[*models.UserSignup](req)
	if err != nil {
		logger.Error("Could not decode submitted signup request.",
			slog.Any("error", err))
		return
	}

	if !userSignup.Valid() {
		ctx := layouts.UserSignupToCtx(req.Context(), userSignup)
		if err := htmx.NewResponse().RenderTempl(ctx, res, layouts.SignUpForm()); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		return
	}

	// spew.Dump(userSignup)
	// // Create the user in the auth backend.
	// userID, err := s.API.user.Create(req.Context(), newUser)
	// if err != nil {
	// 	logging.FromContext(req.Context()).
	// 		Error("Could not create user account.", slog.Any("error", err))

	// 	if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
	// 		logging.FromContext(req.Context()).
	// 			Error("Cannot render sign up error.", slog.Any("error", err))
	// 		res.WriteHeader(http.StatusInternalServerError)
	// 	}

	// 	return
	// }

	// // Create new user in the database backend.
	// err = s.API.elastic.AddUser(req.Context(), userID)
	// if err != nil {
	// 	logging.FromContext(req.Context()).
	// 		Error("Could not create user account.", slog.Any("error", err))

	// 	if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
	// 		logging.FromContext(req.Context()).
	// 			Error("Cannot render sign up error.", slog.Any("error", err))
	// 		res.WriteHeader(http.StatusInternalServerError)
	// 	}
	// }

	// // Show success message.
	// if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupSuccess()); err != nil {
	// 	logging.FromContext(req.Context()).
	// 		Error("Cannot render sign up error.", slog.Any("error", err))
	// 	res.WriteHeader(http.StatusInternalServerError)
	// }
}

func (s Server) PostSignupValidate(res http.ResponseWriter, req *http.Request) {
	// ctx := logging.ToContext(req.Context(), s.Logger.With(slog.String("handler", "Signup")))

	// forms.Validate(res, req.WithContext(ctx), partials.UpdateSignupInput)
}
