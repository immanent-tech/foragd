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

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// ImportSubscriptions contains the data for rendering a page for importing subscriptions.
type ImportSubscriptions struct {
	title    templates.PageTitle
	template templ.Component
	svc      pageServices
}

func (h *ImportSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			h.svc.appCfg,
			h.svc.sessionMgr,
			h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *ImportSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
}

func (m *Manager) HandleSetupImport(subSvc SubscriptionsService, userSvc UserService) http.HandlerFunc {
	return m.ValidateSubscriptionLimits(userSvc, subSvc)(
		http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			user := models.UserFromCtx(req.Context())
			if user == nil {
				slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
					slog.Any("error", models.ErrCtxValueNotFound))
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}
			RenderInternalPage(&ImportSubscriptions{
				title: templates.PageTitle{
					Summary:     "Import",
					Description: "Choose source to import subscriptions",
				},
				template: templates.ImportSubscriptions(),
				svc:      m.NewPageServices(),
			}).ServeHTTP(res, req)
		}),
	).ServeHTTP
}

func (m *Manager) HandleStartImport(importSvc Importer) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract OPML file.
		opmlUpload, err := decodeMultipartFile(req, "source")
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
		RenderPartial(&Notification{
			msg: models.NewInfoMessage(
				"Import started.",
				"Watch the status page for updates.",
			),
		}).ServeHTTP(res, req)

		// Redirect to the status page.
		htmx.LocationResponse(
			htmx.WithLocationPath("/import/status/"+jobID),
			htmx.WithLocationTarget(templates.ContentID.Target()),
			htmx.WithLocationSwap("morph:innerHTML transition:true"),
			htmx.WithLocationHeaders(map[string]string{
				models.ActionHeader: "import-started",
			}),
		).ServeHTTP(res, req)
	}
}

type ImportStatus struct {
	title    templates.PageTitle
	template templ.Component
	svc      pageServices
}

func (p *ImportStatus) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			p.svc.appCfg,
			p.svc.sessionMgr,
			p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

func (p *ImportStatus) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
}

func (m *Manager) HandleImportStatus(importSvc Importer) http.HandlerFunc {
	page := &ImportSubscriptions{
		title: templates.PageTitle{
			Summary: "Import Status",
		},
		svc: m.NewPageServices(),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		jobID := chi.URLParam(req, "jobID")
		status, results, err := importSvc.GetImportStatus(req.Context(), jobID)
		if err != nil {
			var msg *models.UserMessage
			if apiError, ok := errors.AsType[*models.APIError](err); ok && apiError.UserMessage != nil {
				msg = apiError.UserMessage
			} else {
				msg = models.NewErrorMessage(
					"Unexpected error with import",
					"An error occurred processing the import. This might be temporary, please try again",
				)
			}
			page.template = templates.ImportStatus(nil, nil, msg)
			RenderInternalPage(page).ServeHTTP(res, req)
			return
		}
		// If no job ID was given, replace the URL with a new one containing the retrieved job ID.
		if jobID == "" {
			res.Header().Set(htmx.HeaderReplaceUrl, "/import/status"+status.GetID())
		}
		page.template = templates.ImportStatus(status, results, nil)
		page.title.Description = string(status.Status)
		switch status.Status {
		case "pending", "running", "done":
			RenderInternalPage(page).ServeHTTP(res, req)
		case "error":
			slogctx.Error(req.Context(), "Import failed.", slog.Any("error", err))
			RenderInternalPage(page).ServeHTTP(res, req)
		}
	}
}
