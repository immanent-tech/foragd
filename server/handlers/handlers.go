// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/forms"
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
	request, valid, err := forms.DecodeForm[T](req)
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
	if !valid {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusUnprocessableEntity,
			UserMessage: models.NewErrorMessage(
				"Invalid data submitted",
				"Please check your inputs and try again.",
			),
		}
	}
	return request, nil
}

func parseMultipartForm[T forms.FormInput](req *http.Request) (T, error) {
	request, valid, err := forms.DecodeMultiPartForm[T](req)
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
	if !valid {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusUnprocessableEntity,
			UserMessage: models.NewErrorMessage(
				"Invalid data submitted",
				"Please check your inputs and try again.",
			),
		}
	}
	return request, nil
}
