// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"

	"github.com/googleapis/gax-go/v2/apierror"

	"github.com/immanent-tech/foragd/models"
)

// APIError wraps the error returned by a GCP API endpoint in a models.APIError.
func APIError(desc string, err error) error {
	var apiErr *apierror.APIError
	if ok := errors.As(err, &apiErr); ok {
		// ae.HTTPCode() is the HTTP status code.
		// ae.GRPCStatus().Code() is the gRPC status code
		return models.NewAPIError(
			apiErr.HTTPCode(),
			fmt.Errorf(
				"%s: HTTPCode: %d, GRPCStatusCode: %s\n%v",
				desc,
				apiErr.HTTPCode(),
				apiErr.GRPCStatus().Code(),
				apiErr.Message(),
			),
		)
	}
	var googErr *googleapi.Error
	if ok := errors.As(err, &googErr); ok {
		// e.Code is the HTTP status code.
		// e.Message is the error message.
		// e.Body is the raw response body.
		// e.Header contains the HTTP response headers.
		return models.NewAPIError(googErr.Code,
			fmt.Errorf("%s: HTTPCode: %d: %s\n%v", desc, googErr.Code, googErr.Message, googErr.Body),
		)
	}
	return models.NewAPIError(
		http.StatusInternalServerError,
		fmt.Errorf("%s: %w", desc, err),
	)
}
