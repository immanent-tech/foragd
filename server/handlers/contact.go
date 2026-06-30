// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	"github.com/justinas/alice"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

type Contact struct{}

func (p *Contact) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := templates.PageTitle{
		Summary:     "Contact",
		Description: "Contact the developers of Foragd.",
	}
	contactOG := opengraph.NewWebSite(
		title.String(),
		config.GetBaseURL()+"/contact",
		title.Description,
		config.GetBaseURL()+"/content/logo-vertical-light.webp",
	)
	contactJsonLd := schemaorg.NewWebPage(
		config.GetBaseURL()+"/contact",
		title.Summary,
		title.Description,
		title.Description,
		"",
		"",
		"en",
		config.GetBaseURL(),
		"",
		config.GetBaseURL()+"/content/logo-vertical-light.webp",
		"",
		"",
	)
	templ.Handler(templates.CreatePage(templates.Contact(),
		templates.WithPageTitle(title),
		templates.WithPageDescription(title.Description),
		templates.WithOpenGraphMetadata(contactOG),
		templates.WithJSONLDSchema(
			websiteJsonLd,
			contactJsonLd,
			orgJsonLd,
		),
	)).ServeHTTP(res, req)
}

func HandleContact() http.HandlerFunc {
	return RenderExternalPage(&Contact{})
}

func HandleSubmitContact() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeMultiPartForm[*models.ContactRequest](req)
		if err != nil || !valid {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage:   models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
				}).ServeHTTP(res, req)
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
			HandleExternalError(&models.APIError{
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
	}).ServeHTTP
}
