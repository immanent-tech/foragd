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

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

type PageIssue struct {
	template templ.Component
}

// FullResponse renders a full page (headers, footers and content).
func (t *PageIssue) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(t.template,
			templates.WithPageTitle("Report Page Issue"),
		)).ServeHTTP(res, req)
}

// PartialResponse renders just the content and performs OOB swaps to update the title (if set) and sidebar/dock.
func (t *PageIssue) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(t.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Report Page Issue")).ServeHTTP(res, req)
}

// HandleReportPageIssue handles presenting a form for the user to submit issues about the app.
func HandleReportPageIssue() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Display the report issue form.
		RenderInternalPage(&PageIssue{
			template: templates.ReportPageIssue(
				&models.ReportIssueRequest{PageUrl: currentURL, UserEmail: user.GetEmail()},
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSubmitPageIssue handles processing the user submitted subscription issues form.
func HandleSubmitPageIssue() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeMultiPartForm[*models.ReportIssueRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage:   models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
			}).ServeHTTP(res, req)
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: err,
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
		}
		if screenshotURL != "" {
			request.ScreenshotURL = &screenshotURL
		}

		// Create the issue in Github.
		err = github.Connect(req.Context())
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to connect to github: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
		}
		err = github.CreateIssue(req.Context(), request)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("create github issue: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
		}

		// Show notification of issue reported.
		RenderPartial(&Notification{
			msg: models.NewInfoMessage(
				"Thanks for reporting the issue!",
				"We will look into it and implement fixes as appropriate.",
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

type ObjectIssue struct {
	template templ.Component
}

// FullResponse renders a full page (headers, footers and content).
func (t *ObjectIssue) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(t.template,
			templates.WithPageTitle("Report An Issue"),
		)).ServeHTTP(res, req)
}

// PartialResponse renders just the content and performs OOB swaps to update the title (if set) and sidebar/dock.
func (t *ObjectIssue) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(t.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Report An Issue")).ServeHTTP(res, req)
}

// HandleReportObjectIssue presents a form for entering issues about a particular object (subscription/article).
func HandleReportObjectIssue() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		if err := params.Valid(); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode object: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}

		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}

		RenderInternalPage(&ObjectIssue{
			template: templates.ReportObjectIssues(
				string(params.Object),
				params.ObjectID,
				models.NewObjectIssue(params, user.GetEmail(), currentURL),
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSubmitObjectIssue handles processing the issue form and creating a github issue with the details.
func HandleSubmitObjectIssue() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the issue request details.
		request, valid, err := forms.DecodeMultiPartForm[*models.ReportObjectIssueRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode object: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to generate issues form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: err,
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
		}
		if screenshotURL != "" {
			request.ScreenshotURL = &screenshotURL
		}

		err = github.Connect(req.Context())
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("connect to github: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}
		// Create the issue in Github.
		err = github.CreateObjectIssue(req.Context(), request)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("create github issue: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit issue",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}

		// Show notification of issue reported.
		RenderPartial(&Notification{
			msg: models.NewInfoMessage(
				"Thanks for reporting the issue!",
				"We will look into it and implement fixes as appropriate.",
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
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
		imageFileID := strconv.FormatUint(xxh3.Hash([]byte(image.Header.Filename)), 10)
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
