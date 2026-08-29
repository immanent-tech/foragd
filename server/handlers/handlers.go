// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
)

type Route = string

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

// setRedirect adds the HX-Location header with the given values to the response, which triggers a client side
// redirection without reloading the whole page.
//
// https://htmx.org/headers/hx-location/
func setRedirect(res http.ResponseWriter, request htmx.HXLocationRequest) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("set redirect: marshal request: %w", err)
	}
	res.Header().Set(htmx.HeaderLocation, string(requestJSON))
	return nil
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
