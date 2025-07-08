// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/views"
)

// NewSubscriptionRequestResult handles processing the result of an add subscription request.
func NewSubscriptionRequestResult(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// If there is a response object in the context, a backend error occurred and the add subscription request failed.
		_, found := req.Context().Value(respCtxKey).(*models.Response)
		if found {
			var request *models.SubscriptionRequest
			requests := subscriptionRequestsFromCtx(req.Context())
			if requests != nil {
				request = requests[0]
			}
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while adding the subscription.",
			}
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(request, msg))
			next.ServeHTTP(res, req.WithContext(ctx))
		}
		// Extract the processed request from the context.
		results := subscriptionResultsFromCtx(req.Context())
		if results == nil {
			next.ServeHTTP(res, req)
			return
		}
		// Display the modal with the request results shown.
		for _, result := range results {
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(&models.SubscriptionRequest{}, result))
			next.ServeHTTP(res, req.WithContext(ctx))
			break
		}
	})
}

// NewSubscriptionsImport handles setting up a new subscription import process for the user.
func NewSubscriptionsImport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templateToCtx(req.Context(), views.ImportSubscriptionLayout())
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ProcessSubscriptionsImport handles both setting up the method and actioning the method used for the subscription
// import process.
func ProcessSubscriptionsImport(importMethod string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			switch req.Method {
			case http.MethodPut:
				switch importMethod {
				case "opml_file":
					ctx = templateToCtx(ctx, views.ImportFromOPML())
				}
			case http.MethodPost:
				switch importMethod {
				case "opml_file":
					// Decode the OPML file form input.
					opmlFile := &models.OPMLFile{}
					opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
					if err != nil || !valid {
						ctx = context.WithValue(ctx, respCtxKey,
							models.NewResponse(http.StatusNotAcceptable, fmt.Errorf("could not parse OPML file: %w", err)))
						return
					}
					opmlImport, err := opmlFile.Parse()
					if err != nil {
						ctx = context.WithValue(ctx, respCtxKey,
							models.NewResponse(http.StatusNotAcceptable, fmt.Errorf("could not parse OPML file: %w", err)))
						return
					}
					// Extract the individual feeds from the OPML object and create a subscription
					// request for each one.
					feeds := opmlImport.ExtractRSS()
					requests := make([]*models.SubscriptionRequest, 0, len(feeds))
					for _, feed := range feeds {
						requests = append(requests, &models.SubscriptionRequest{URL: feed.XMLURL})
					}
					// Create a map of requests to their individual results.
					results := make(map[*models.SubscriptionRequest]*models.Response)
					ctx = context.WithValue(ctx, subscriptionResultsCtxKey, results)
					// Store requests in context.
					ctx = context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
				}
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SubscriptionsImportResults handles showing the result of a subscriptions import.
func SubscriptionsImportResults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// If there is a response object in the context, a backend error occurred and the add subscription request failed.
		_, found := req.Context().Value(respCtxKey).(*models.Response)
		if found {
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while importing subscriptions.",
			}
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(&models.SubscriptionRequest{}, msg))
			next.ServeHTTP(res, req.WithContext(ctx))
		}

		// Get the request.
		results := subscriptionResultsFromCtx(req.Context())
		if results == nil {
			next.ServeHTTP(res, req)
			return
		}
		ctx := templateToCtx(req.Context(), views.ImportResults(results))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func NewUserSignup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templateToCtx(req.Context(), views.SignUpPage(models.NewUserSignup()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func ProcessUserSignup(userBackendAPI models.UserBackendAPI, userFrontendAPI models.DocumentsAPI, signupRequest *models.UserSignupRequest) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Create the user in the auth backend.
			userID, err := userBackendAPI.Create(req.Context(), signupRequest)
			if err != nil {
				ProcessResponse(res, req,
					models.NewResponse(http.StatusInternalServerError, fmt.Errorf("failed to update user: %w", err)))
				return
			}
			// Create new user in the database backend.
			ctx := elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix)
			err = userFrontendAPI.AddUser(ctx, userID)
			if err != nil {
				ProcessResponse(res, req,
					models.NewResponse(http.StatusInternalServerError, fmt.Errorf("failed to update user: %w", err)))
				return
			}
			signupRequest.Msg = &models.UserMessage{
				Status:  models.UserMessageStatusSuccess,
				Summary: "Account created!",
			}
			ctx = templateToCtx(ctx, views.SignupForm(signupRequest))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
