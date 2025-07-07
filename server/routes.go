// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/views"
)

var ErrInvalidParam = errors.New("invalid parameter")

// SignUp handles presenting a form for the user to enter sign-up details.
func (s Server) SignUp(res http.ResponseWriter, req *http.Request) {
	alice.New(
		handlers.NewUserSignup,
	).Then(handlers.RenderTemplate()).ServeHTTP(res, req)
}

// ProcessSignUp handles validating and processing a user sign-up request.
func (s Server) ProcessSignUp(res http.ResponseWriter, req *http.Request) {
	resp := htmx.NewResponse()
	// Decode and validate the user sign-up request.
	userSignup, valid, err := forms.DecodeForm[*models.UserSignupRequest](req)
	if err != nil || !valid {
		slogctx.FromCtx(req.Context()).Debug("Problem decoding user signup form data.",
			slog.Bool("valid", valid),
			slog.Any("error", err),
		)
		if err := resp.RenderTempl(req.Context(), res, views.SignupForm(userSignup)); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Bad request.", slog.Any("error", err))
			http.Error(res, "user signup failed!", http.StatusInternalServerError)
		}
		return
	}
	// Process the sign-up request.
	alice.New(
		handlers.RouteLogger,
		handlers.ProcessUserSignup(s.UserAPI(), s.DataAPI(), userSignup),
	).Then(handlers.RenderTemplate()).ServeHTTP(res, req)
}

func (s Server) ActionArticles(res http.ResponseWriter, req *http.Request, action Action, params ActionArticlesParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

// AddSubscription handles an add subscription request.
func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	// Add requests are only driven by htmx requests.
	if !htmx.IsHTMX(req) {
		handlers.ProcessResponse(res, req, models.RespForbidden(models.ErrHTMXRequired))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ParseNewSubscriptionRequest,
		handlers.MatchFeedsToSubscriptionRequests(s.DataAPI()),
		handlers.AddFeedsForSubscriptionRequests(s.DataAPI()),
		handlers.AddSubscriptions(s.DataAPI()),
		handlers.NewSubscriptionRequestResult,
	).Then(handlers.RenderTemplate()).ServeHTTP(res, req)
}

// // StartSubscriptionImport handles starting a subscriptions import process for the user.
// func (s Server) StartSubscriptionImport(res http.ResponseWriter, req *http.Request) {
// 	chain := alice.New(
// 		handlers.RouteLogger,
// 		handlers.NewSubscriptionsImport,
// 	)

// 	switch htmx.IsHTMX(req) {
// 	case true:
// 		chain.Then(handlers.RenderTemplate()).ServeHTTP(res, req)
// 	case false:
// 		chain.Append(handlers.GenerateDrawerContent(s.DataAPI())).Then(handlers.RenderTemplate()).ServeHTTP(res, req)
// 	}
// }

func (f *SetSubscriptionImportMethodFormdataBody) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false, err
	}
	return true, nil
}

func (f *SetSubscriptionImportMethodFormdataBody) Sanitise() error {
	return nil
}

// SetSubscriptionImportMethod handles setting the method that will be used for importing susbcriptions from the user's
// choice.
func (s Server) SetSubscriptionImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetSubscriptionImportMethodFormdataBody](req)
	if err != nil || !valid {
		handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		return
	}

	alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(string(importMethod.From)),
	).Then(handlers.RenderTemplate()).ServeHTTP(res, req)
}

// ProcessSubscriptionImport handles using the user's chosen import method to import their subscriptions.
func (s Server) ProcessSubscriptionImport(res http.ResponseWriter, req *http.Request) {
	// Decode the import source.
	importMethod, err := forms.DecodeMultipartValue(req, "source")
	if err != nil {
		handlers.ProcessResponse(res, req, models.RespErrBackend(err))
		return
	}

	chain := alice.New(
		handlers.RouteLogger,
		handlers.ProcessSubscriptionsImport(importMethod),
		handlers.MatchFeedsToSubscriptionRequests(s.DataAPI()),
		handlers.AddFeedsForSubscriptionRequests(s.DataAPI()),
		handlers.AddSubscriptions(s.DataAPI()),
		handlers.SubscriptionsImportResults,
	).Then(handlers.RenderTemplate())
	chain.ServeHTTP(res, req)
}

func (s Server) SetupSubscriptionsExport(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) PerformSubscriptionsExport(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
