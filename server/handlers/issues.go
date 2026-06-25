// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
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
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Report Page Issue")).ServeHTTP(res, req)
}

// HandleReportIssue handles presenting a form for the user to submit issues about the app.
func HandleReportIssue() http.HandlerFunc {
	return internalPageHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to generate issues form",
						"There was a problem with the request. Please try again.",
					),
				}).ServeHTTP(res, req)
		}

		objectID := req.FormValue("object_id")

		// Display the report issue form.
		RenderInternalPage(&PageIssue{
			template: templates.ReportIssue(
				&models.ReportIssueRequest{PageUrl: req.Referer(), UserEmail: user.GetEmail(), ObjectID: &objectID},
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSubmitIssue handles processing the user submitted subscription issues form.
func HandleSubmitIssue() http.HandlerFunc {
	return internalPageHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeMultiPartForm[*models.ReportIssueRequest](req)
		if err != nil || !valid {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage:   models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
				}).ServeHTTP(res, req)
			return
		}

		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to generate issues form",
						"There was a problem with the request. Please try again.",
					),
				}).ServeHTTP(res, req)
			return
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(req)
		if err != nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: err,
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to submit issue",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
			return
		}
		if screenshotURL != "" {
			request.ScreenshotURL = &screenshotURL
		}

		// Build issue body.
		var bodyBuilder strings.Builder
		bodyBuilder.WriteString("User ID: ")
		bodyBuilder.WriteString(user.GetID())
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString("Contact Email: ")
		bodyBuilder.WriteString(request.UserEmail)
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString("Page URL: ")
		bodyBuilder.WriteString(request.PageUrl)
		bodyBuilder.WriteRune('\n')
		if request.ObjectID != nil && *request.ObjectID != "" {
			bodyBuilder.WriteString("Object ID: ")
			bodyBuilder.WriteString(*request.ObjectID)
			bodyBuilder.WriteRune('\n')
		}
		if request.Details != nil {
			bodyBuilder.WriteRune('\n')
			bodyBuilder.WriteString("Details:")
			bodyBuilder.WriteRune('\n')
			bodyBuilder.WriteString(*request.Details)
			bodyBuilder.WriteRune('\n')
		}
		if request.ScreenshotURL != nil {
			bodyBuilder.WriteString("![](")
			bodyBuilder.WriteString(screenshotURL)
			bodyBuilder.WriteString(")")
			bodyBuilder.WriteRune('\n')
		}

		if err := resend.SendEmail(req.Context(),
			resend.WithFrom[*resend.Email]("no-reply@foragd.app"),
			resend.WithReplyTo[*resend.Email](request.UserEmail),
			resend.WithTo("support@immanent.tech"),
			resend.WithSubject[*resend.Email]("Issue reported by "+user.GetNickname()),
			resend.WithTextContent(bodyBuilder.String()),
			resend.WithTag(resend.TagCategory, resend.TagCategorySupport),
			resend.WithTag(resend.TagUserID, user.GetID()),
			resend.WithRemoteAttachment(
				resend.NewRemoteFileAttachment(screenshotURL, filepath.Base(screenshotURL))),
		); err != nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("connect to github: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to submit issue",
						"There was a problem with the request. Please try again.",
					),
				}).ServeHTTP(res, req)
			return
		}

		// Redirect back to the referring page.
		// res.Header().Add(htmx.HeaderRedirect, request.PageUrl)

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
		// Generate a unique ID for the avatar image in the cache using the user ID.
		imageFileID := strconv.FormatUint(xxh3.Hash([]byte(image.Header.Filename)), 10)
		// Read the uploaded data and store in the cache.
		imageData, err := io.ReadAll(image.Data)
		if err != nil {
			return "", fmt.Errorf("read image: %w", err)
		}
		if err := cache.SaveScreenshot(req.Context(), imageFileID, imageData); err != nil {
			return "", fmt.Errorf("save screenshot: %w", err)
		}
		// Construct a new full URL to the uploaded avatar on the local server.
		return config.GetBaseURL() + "/img/screenshot/" + imageFileID, nil
	}
	return "", nil
}
