// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"net/http"
)

// (GET /user/new)
func (s Server) NewUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// Save a new user.
// (PUT /user/new)
func (s Server) AddUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// // SignUp serves the user sign up page.
// // GET(/signup).
// func (s Server) Signup(res http.ResponseWriter, req *http.Request) {
// 	var page templ.Component

// 	logger := s.Logger.With(slog.String("handler", "Signup"))

// 	ctx := layouts.UserSignupToCtx(req.Context(), models.NewUserSignup())

// 	page = layouts.Page("Go Feed Me - Sign-up",
// 		layouts.WithPageDescription("Sign-up to Go Feed Me."),
// 		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
// 		layouts.WithPageContent(layouts.SignupLayout()))

// 	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
// 		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
// 		http.Error(res, "Problem!", http.StatusInternalServerError)
// 	}
// }

// // PostSignup processes the user sign up request.
// // POST(/signup).
// func (s Server) ProcessSignup(res http.ResponseWriter, req *http.Request) {
// 	logger := logging.NewHandlerLogger("ProcessSignup", req)

// 	userSignup, valid, err := forms.DecodeForm[*models.APIUserSignupRequest](req)
// 	if err != nil {
// 		logger.Error("Could not decode submitted signup request.",
// 			slog.Any("error", err))
// 		return
// 	}

// 	if !valid {
// 		ctx := layouts.UserSignupToCtx(req.Context(), userSignup)
// 		if err = htmx.NewResponse().RenderTempl(ctx, res, userSignup.Form()); err != nil {
// 			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
// 			http.Error(res, "Problem!", http.StatusInternalServerError)
// 		}

// 		return
// 	}

// 	// Create the user in the auth backend.
// 	userID, err := s.API.user.Create(req.Context(), userSignup)
// 	if err != nil {
// 		logger.Error("Could not create user account.", slog.Any("error", err))

// 		if err = htmx.NewResponse().RenderTempl(req.Context(), res, layouts.SignupError()); err != nil {
// 			logger.Error("Cannot render sign up error.", slog.Any("error", err))
// 			res.WriteHeader(http.StatusInternalServerError)
// 		}

// 		return
// 	}

// 	// Create new user in the database backend.
// 	addUserCtx := elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix)

// 	err = s.API.elastic.AddUser(addUserCtx, userID)
// 	if err != nil {
// 		logger.Error("Could not create user account.", slog.Any("error", err))

// 		if err = htmx.NewResponse().RenderTempl(req.Context(), res, layouts.SignupError()); err != nil {
// 			logger.Error("Cannot render sign up error.", slog.Any("error", err))
// 			res.WriteHeader(http.StatusInternalServerError)
// 		}
// 	}

// 	// Show success message.
// 	if err := htmx.NewResponse().RenderTempl(req.Context(), res, layouts.SignupSuccess()); err != nil {
// 		logger.Error("Cannot render sign up error.", slog.Any("error", err))
// 		res.WriteHeader(http.StatusInternalServerError)
// 	}
// }
