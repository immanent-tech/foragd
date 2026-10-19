// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/web/templates"
)

type PageIssue struct {
	*Page
}

// HandleReportIssue handles presenting a form for the user to submit issues about the app.
func (m *Manager) HandleReportIssue() http.HandlerFunc {
	page := &PageIssue{
		Page: m.NewPage("/issue",
			WithSummary("Report Issue"),
			WithDescription("Report issues and problems with the site"),
			WithMetadataDescription("Report issues and problems with Foragd"),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		objectID := req.FormValue("object_id")

		page.template = templates.ReportIssue(
			&models.ReportIssueRequest{PageUrl: req.Referer(), UserEmail: user.GetEmail(), ObjectID: &objectID},
			m.Breadcrumbs,
		)

		// Display the report issue form.
		page.Render().ServeHTTP(res, req)
	}
}

// HandleSubmitIssue handles processing the user submitted subscription issues form.
func (m *Manager) HandleSubmitIssue(cache ImageCache, emailSender EmailSender) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Validate the subscription issue request.
		request, err := parseMultipartForm[*models.ReportIssueRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Process any uploaded screenshot.
		screenshotURL, err := processScreenshots(m.AppConfig, cache, req)
		if err != nil {
			m.HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
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

		if err := emailSender.Send(req.Context(),
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
			m.HandleInternalError(http.StatusInternalServerError, fmt.Errorf("send email: %w", err)).ServeHTTP(res, req)
			return
		}

		// Redirect back to the referring page.
		// res.Header().Add(htmx.HeaderRedirect, request.PageUrl)

		// Show notification of issue reported.
		m.HandleNotification(res, req,
			models.NewInfoMessage(
				"Thanks for reporting the issue!",
				"We will look into it and implement fixes as appropriate.",
			),
		)
	}
}

// processScreenshots handles processing an uploaded screenshot file, storing in the server cache and generating a
// unique URL to reference the cached file.
func processScreenshots(appCfg AppConfig, cache ImageCache, req *http.Request) (string, error) {
	const maxScreenshotSize = 10000000 // Max screenshot size is 10 MB.

	// Get any uploaded screenshot.
	image, err := forms.DecodeMultipartFile(req, "screenshot")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return "", fmt.Errorf("decode screenshot upload: %w", err)
	}
	if image.GetSize() > maxScreenshotSize {
		return "", fmt.Errorf("decode screenshot upload: %w", forms.ErrFileTooLarge)
	}
	// If the user uploaded a new avatar, process it.
	if image != nil {
		// Generate a unique ID for the avatar image in the cache using the user ID.
		imageFileID := strconv.FormatUint(xxh3.Hash([]byte(image.GetFilename())), 10)
		// Read the uploaded data and store in the cache.
		imageData, err := io.ReadAll(image)
		if err != nil {
			return "", fmt.Errorf("read image: %w", err)
		}
		if err := cache.SaveScreenshot(req.Context(), imageFileID, imageData); err != nil {
			return "", fmt.Errorf("save screenshot: %w", err)
		}
		// Construct a new full URL to the uploaded avatar on the local server.
		return appCfg.GetBaseURL().JoinPath("/img/screenshot/" + imageFileID).String(), nil
	}
	return "", nil
}
