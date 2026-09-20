/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-base/server/forms"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	"go.opentelemetry.io/otel"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Route = string

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/server/handlers")

// RedirectTo performs route redirection for routes that have moved.
func RedirectTo(target string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dest := target
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, code)
	}
}

// RedirectParam performs route redirection for routes with parameters that have moved.
func RedirectParam(paramName, targetTmpl string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := chi.URLParam(r, paramName)
		dest := fmt.Sprintf(targetTmpl, val)
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, code)
	}
}

func parseForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

func parseMultipartForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeMultiPartForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

// FileUpload represents file data uploaded through a mutlipart form.
type FileUpload interface {
	Set(hdr *multipart.FileHeader, data multipart.File)
}

// DecodeMultipartFile will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func decodeMultipartFile(req *http.Request, field string) (*models.FileUpload, error) {
	// defaultMaxSize for a multipart for submission is 32 MB.
	const defaultMaxSize = 32 << 20

	// Parse form values in request.
	if err := req.ParseMultipartForm(defaultMaxSize); err != nil {
		return nil, fmt.Errorf("decode multipart form: %w", err)
	}
	// Decode the form values.
	data, hdr, err := req.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("decode form file: %w", err)
	}
	// Create a models.FileUpload object.
	upload := &models.FileUpload{
		Data:   data,
		Header: hdr,
	}
	// Validate file upload.
	if err := upload.Validate(); err != nil {
		return nil, fmt.Errorf("validate file upload: %w", err)
	}
	return upload, nil
}

// PostHandlerHook is a function that can be run after a handler has done its main processing. Used mainly to perform
// route-specific or other conditional logic without complicating the handler code.
type PostHandlerHook func(res http.ResponseWriter, req *http.Request) error

type pageMetadata struct {
	Title       templates.PageTitle
	Description string
	Path        string
	ImagePath   string
}

func (m pageMetadata) CanonicalLink(req *http.Request) string {
	baseURL := req.URL.Clone()
	baseURL.Path = "/"
	return baseURL.JoinPath(m.Path).String()
}

func (m pageMetadata) OpengraphData(req *http.Request) *opengraph.WebSite {
	baseURL := req.URL.Clone()
	baseURL.Path = "/"
	return opengraph.NewWebSite(
		m.Title.String(),
		baseURL.JoinPath(m.Path).String(),
		m.Description,
		baseURL.JoinPath(m.ImagePath).String(),
	)
}

func (m pageMetadata) JSONLD(req *http.Request) *schemaorg.WebPage {
	baseURL := req.URL.Clone()
	baseURL.Path = "/"
	return schemaorg.NewWebPage(
		baseURL.JoinPath(m.Path).String(),
		m.Title.Summary,
		m.Title.Description,
		m.Description,
		"",
		"",
		"en",
		baseURL.String(),
		"",
		baseURL.JoinPath(m.ImagePath).String(),
		"",
		"",
	)
}

func generateSiteJSONLD(req *http.Request) *schemaorg.WebSite {
	baseURL := req.URL.Clone()
	baseURL.Path = "/"
	return schemaorg.NewWebSite(
		baseURL.String(),
		"Foragd",
		"Foragd RSS and Atom Feed Reader",
		"Foragd is a web-based RSS and Atom Feed Reader with a responsive design, no ads and no algorithm directing you.",
		nil,
	)
}

func generateSiteOG(req *http.Request) *opengraph.WebSite {
	baseURL := req.URL.Clone()
	baseURL.Path = "/"
	return opengraph.NewWebSite(
		"Foragd",
		baseURL.String(),
		"Foragd is a web-based RSS and Atom Feed Reader with a responsive design, no ads and no algorithm directing you.",
		baseURL.JoinPath("/content/logo-vertical-light.webp").String(),
	)
}
