// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/angelofallars/htmx-go"
	"github.com/cespare/xxhash/v2"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// GetPageIssues handles presenting a form for the user to submit issues about the app.
func GetPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Display the report issue form.
		template := templates.ReportPageIssue(
			&models.ReportIssueRequest{PageUrl: currentURL, UserEmail: user.GetEmail()},
		)
		ctx := templates.PageTitleToCtx(req.Context(), "Report Page Issue")
		renderPage(wrapContent(req, template)).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SubmitPageIssues handles processing the user submitted subscription issues form.
func SubmitPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.ReportIssueRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage:   models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
			}
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(req)
		if err != nil {
			return &models.APIError{
				InternalError: err,
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if screenshotURL != "" {
			request.ScreenshotURL = screenshotURL
		}

		// Create the issue in Github.
		err = github.Connect(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("unable to connect to github: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		err = github.CreateIssue(req.Context(), request)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("create github issue: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		if err := params.Valid(); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode object: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract the issue request details.
		request, valid, err := forms.DecodeForm[*models.ReportObjectIssueRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("decode object: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(req)
		if err != nil {
			return &models.APIError{
				InternalError: err,
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if screenshotURL != "" {
			request.ScreenshotURL = screenshotURL
		}

		err = github.Connect(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("connect to github: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		// Create the issue in Github.
		err = github.CreateObjectIssue(req.Context(), request)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("create github issue: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"There was a problem with the request. Please try again.",
				),
			}
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

// processScreenshots handles processing an uploaded screenshot file, storing in the server cache and generating a
// unique URL to reference the cached file.
func processScreenshots(req *http.Request) (string, error) {
	const maxScreenshotSize = 10000000 // Max screenshot size is 10 MB.

	// Get any uploaded screenshot.
	image, err := forms.DecodeMultipartFile(req, "screenshot")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return "", fmt.Errorf("decode screenshot upload: %w", err)
	}
	if image.GetSize() > maxScreenshotSize {
		return "", fmt.Errorf("decode screenshot upload: %w", models.ErrFileTooLarge)
	}
	// If the user uploaded a new avatar, process it.
	if image != nil {
		screenshotCache, err := loadScreenshotCache()
		if err != nil {
			return "", fmt.Errorf("load screenshot cache: %w", err)
		}
		// Generate a unique ID for the avatar image in the cache using the user ID.
		imageFileID := strconv.FormatUint(xxhash.Sum64String(image.Header.Filename), 10)
		// Read the uploaded data and store in the cache.
		imageData, err := io.ReadAll(image.Data)
		if err != nil {
			return "", fmt.Errorf("read image: %w", err)
		}
		screenshotCache.Set(req.Context(), imageFileID, imageData)
		// Construct a new full URL to the uploaded avatar on the local server.
		baseURL := os.Getenv("FORAGD_BASEURL")
		return baseURL + "/img/screenshot/" + imageFileID, nil
	}
	return "", nil
}
