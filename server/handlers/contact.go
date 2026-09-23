// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/web/templates"
)

type Contact struct {
	template templ.Component
}

func (p *Contact) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template).ServeHTTP(res, req)
}

func (m *Manager) HandleContact() http.HandlerFunc {
	metadata := pageMetadata{
		Title: templates.PageTitle{
			Summary:     "Contact",
			Description: "Contact the developers of Foragd.",
		},
		Description: "Contact the developers of Foragd.",
		Path:        "/contact",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
	}
	return RenderExternalPage(&Contact{
		template: templates.CreatePage(
			m.AppConfig,
			m.SessionMgr,
			templates.Contact(),
			templates.WithPageTitle(metadata.Title),
			templates.WithPageDescription(metadata.Description),
			templates.WithCanonicalLink(metadata.CanonicalLink()),
			templates.WithOpenGraphMetadata(metadata.OpengraphData()),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(metadata.baseURL),
				metadata.JSONLD(),
				orgJsonLd,
			),
		),
	})
}

func (m *Manager) HandleSubmitContact() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Validate the subscription issue request.
		request, err := parseMultipartForm[*models.ContactRequest](req)
		if err != nil {
			m.HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		// Build issue body.
		var bodyBuilder strings.Builder
		bodyBuilder.WriteString("Contact Email: ")
		bodyBuilder.WriteString(request.ContactEmail)
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString("Details:")
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString(request.Details)
		bodyBuilder.WriteRune('\n')

		if err := resend.SendEmail(req.Context(),
			resend.WithFrom[*resend.Email]("no-reply@foragd.app"),
			resend.WithReplyTo[*resend.Email](request.ContactEmail),
			resend.WithTo("support@immanent.tech"),
			resend.WithSubject[*resend.Email]("Contact form submission from "+request.ContactEmail),
			resend.WithTextContent(bodyBuilder.String()),
			resend.WithTag(resend.TagCategory, resend.TagCategorySupport),
		); err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("send contact email: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to submit form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Show notification of issue reported.
		RenderPartial(&Notification{
			msg: models.NewInfoMessage(
				"Thanks for contacting us!",
				"If we need to reach out to discuss, we will send you an email to the address that was submitted.",
			),
		}).ServeHTTP(res, req)
	}
}
