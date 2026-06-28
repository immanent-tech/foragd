// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/a-h/templ"
	"github.com/justinas/alice"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/web/templates"
)

type ForgetMe struct{}

func (p *ForgetMe) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := templates.PageTitle{
		Summary:     "Forget Me Request",
		Description: "Request removal of your account and personal data",
	}
	description := "Request deletion of your account and personal data."
	templ.Handler(templates.CreatePage(templates.ForgetMe(),
		templates.WithPageTitle(title),
		templates.WithPageDescription(description),
		templates.WithOpenGraphMetadata(opengraph.New(
			title.String(),
			"website",
			config.GetBaseURL()+"/forget-me",
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
		)),
	)).ServeHTTP(res, req)
}

func HandleForgetMe() http.HandlerFunc {
	return RenderExternalPage(&ForgetMe{})
}

func HandleSubmitForgetMe() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		contact, err := mail.ParseAddress(req.FormValue("contact_email"))
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("parse contact address: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Invalid contact address",
					"There was a problem with the contact address entered. Please check, correct if needed and try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Build issue body.
		var bodyBuilder strings.Builder
		bodyBuilder.WriteString("Contact Email: ")
		bodyBuilder.WriteString(contact.String())
		bodyBuilder.WriteRune('\n')

		if err := resend.SendEmail(req.Context(),
			resend.WithFrom[*resend.Email]("no-reply@foragd.app"),
			resend.WithReplyTo[*resend.Email](contact.Address),
			resend.WithTo("privacy@immanent.tech"),
			resend.WithSubject[*resend.Email]("Forget me request from "+contact.String()),
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
				"Your request has been recieved",
				"We will reply via email to confirm and then once the request is complete.",
			),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}
