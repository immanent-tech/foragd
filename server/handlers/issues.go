// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// GetPageIssues handles presenting a form for the user to submit issues about the app.
func GetPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return models.NewAPIError(
				fmt.Errorf("get user data: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Display the report issue form.
		template := templates.ReportPageIssue(&models.IssueRequest{PageUrl: currentURL, UserEmail: user.GetEmail()})
		ctx := templates.PageTitleToCtx(req.Context(), "Report Page Issue")
		renderPage(wrapContent(req, template)).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SubmitPageIssues handles processing the user submitted subscription issues form.
func SubmitPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.IssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Create the issue in Github.
		err = github.Connect(req.Context())
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to connect to github: %w", err),
				http.StatusInternalServerError,
			)
		}
		err = github.CreateIssue(req.Context(), request)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Force refresh of page.
		msg := models.NewErrorMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		ctx := templates.PageTitleToCtx(req.Context(), "Report Page Issue")
		renderPage(wrapContent(req, templates.IssueReportedConfirmation(msg))).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// GetObjectIssues presents a form for entering issues about a particular object (subscription/article).
func GetObjectIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		if err := params.Valid(); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Server could not complete request",
					"This might be temporary, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return models.NewAPIError(
				err,
				http.StatusInternalServerError,
			)
		}

		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		template := templates.ReportObjectIssues(
			string(params.Object),
			params.ObjectID,
			models.NewObjectIssue(params, user.GetEmail(), currentURL),
		)
		ctx := templates.PageTitleToCtx(req.Context(), "Report an issue")
		renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SubmitObjectIssues handles processing the issue form and creating a github issue with the details.
func SubmitObjectIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract the issue request details.
		request, valid, err := forms.DecodeForm[*models.ObjectIssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Server could not complete request",
					"This might be temporary, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// // Extract any attached screenshot.
		// screenshot, err := forms.DecodeMultipartFile2(req, "screenshot")
		// // Validate the screenshot is an image file.
		// mimeType, err := screenshot.ParseMimetype()
		// if err != nil {
		// 	renderPartial(templates.ServerErrorNotification(
		// 		models.NewErrorMessage("Invalid request data", "This might be temporary, please try again."),
		// 	)).ServeHTTP(res, req)
		// 	return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		// }
		// if !slices.Contains([]string{"image/jpeg", "image/png"}, mimeType) {
		// 	renderPartial(templates.ServerErrorNotification(
		// 		models.NewErrorMessage("Invalid request data", "This might be temporary, please try again."),
		// 	)).ServeHTTP(res, req)
		// 	return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		// }

		err = github.Connect(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to connect to github: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Create the issue in Github.
		err = github.CreateObjectIssue(req.Context(), request)
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to submit issue",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to create issue in github: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Force refresh of page.
		msg := models.NewInfoMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		ctx := templates.PageTitleToCtx(req.Context(), "Report issue")
		renderPage(
			wrapContent(req.WithContext(ctx), templates.IssueReportedConfirmation(msg)),
		).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}
