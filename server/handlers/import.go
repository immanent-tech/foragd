/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// ImportSubscriptions contains the data for rendering a page for importing subscriptions.
type ImportSubscriptions struct {
	*Page
}

func (m *Manager) HandleSetupImport(
	subSvc SubscriptionsService,
	userSvc UserService,
	emailSender EmailSender,
) http.HandlerFunc {
	page := &ImportSubscriptions{
		Page: m.NewPage("/import",
			WithSummary("Import"),
			WithDescription("Select an OPML file to import your existing subscriptions"),
			WithTemplate(templates.ImportSubscriptions()),
		),
	}
	return m.ValidateSubscriptionLimits(userSvc, subSvc, emailSender)(
		http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			user := models.UserFromCtx(req.Context())
			if user == nil {
				slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
					slog.Any("error", models.ErrCtxValueNotFound))
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}
			page.Render().ServeHTTP(res, req)
		}),
	).ServeHTTP
}

func (m *Manager) HandleStartImport(importSvc Importer) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract OPML file.
		opmlUpload, err := forms.DecodeMultipartFile(req, "source")
		if err != nil {
			m.HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("decode opml: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Start the import.
		jobID, err := importSvc.StartImport(req.Context(), models.NewOPMLFile(opmlUpload))
		if err != nil {
			if apiError, ok := errors.AsType[*models.APIError](err); ok {
				m.HandleInternalError(apiError.StatusCode, apiError)
			} else {
				m.HandleInternalError(http.StatusInternalServerError, err, WithUserMessage(
					models.NewErrorMessage("Could not start import", ""),
				))
			}
			return
		}

		// Show a notification that the import started.
		m.HandleNotification(res, req,
			models.NewInfoMessage(
				"Import started.",
				"Watch the status page for updates.",
			),
		)

		// Redirect to the status page.
		htmx.LocationResponse(
			htmx.WithLocationPath("/import/status/"+jobID),
			htmx.WithLocationTarget(templates.ContentID.Target()),
			htmx.WithLocationSwap("innerMorph transition:true"),
			htmx.WithLocationHeaders(map[string]string{
				models.ActionHeader: "import-started",
			}),
		).ServeHTTP(res, req)
	}
}

type ImportStatus struct {
	*Page
}

func (m *Manager) HandleImportStatus(importSvc Importer, subSvc SubscriptionsService) http.HandlerFunc {
	page := &ImportSubscriptions{
		Page: m.NewPage("/import/status",
			WithSummary("Import Status"),
			WithDescription("View the status of any import(s)"),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Have jobID URL param.
		if jobID := chi.URLParam(req, "jobID"); jobID != "" {
			// Invalidate the user subscription cache.
			subSvc.Invalidate(user.GetID())

			page.Path = "/import/status/" + jobID

			status, results, err := importSvc.GetImportStatus(req.Context(), jobID)
			if err != nil {
				var msg *models.UserMessage
				if apiError, ok := errors.AsType[*models.APIError](err); ok && apiError.UserMessage != nil {
					msg = apiError.UserMessage
					res.WriteHeader(apiError.StatusCode)
				} else {
					msg = models.NewErrorMessage(
						"Unexpected error with import",
						"An error occurred processing the import. This might be temporary, please try again",
					)
					res.WriteHeader(http.StatusInternalServerError)
				}
				page.template = templates.ImportStatus(nil, nil, msg)
				page.Render().ServeHTTP(res, req)
				return
			}
			// If no job ID was given, replace the URL with a new one containing the retrieved job ID.
			if jobID == "" {
				res.Header().Set(htmx.HeaderReplaceUrl, "/import/status"+status.GetID())
			}
			page.template = templates.ImportStatus(status, results, nil)
			page.Description = string(status.Status)
			switch status.Status {
			case "pending", "running", "done":
				page.Render().ServeHTTP(res, req)
			case "error":
				slogctx.Error(req.Context(), "Import failed.", slog.Any("error", err))
				page.Render().ServeHTTP(res, req)
			}
		} else {
			imports, err := importSvc.GetAllImports(req.Context(), user.GetID())
			if err != nil {
				var msg *models.UserMessage
				if apiError, ok := errors.AsType[*models.APIError](err); ok && apiError.UserMessage != nil {
					msg = apiError.UserMessage
					res.WriteHeader(apiError.StatusCode)
				} else {
					msg = models.NewErrorMessage(
						"Unexpected error with import",
						"An error occurred processing the import. This might be temporary, please try again",
					)
					res.WriteHeader(http.StatusInternalServerError)
				}
				page.template = templates.ImportList(nil, msg)
				page.Render().ServeHTTP(res, req)
				return
			}
			page.template = templates.ImportList(imports, nil)
			page.Render().ServeHTTP(res, req)
		}
	}
}
