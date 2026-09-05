// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"

	"github.com/goforj/godump"
	"github.com/googleapis/gax-go/v2/apierror"

	"github.com/immanent-tech/foragd/models"
)

// APIError wraps the error returned by a GCP API endpoint in a models.APIError.
func APIError(desc string, err error) error {
	if apiErr, ok := errors.AsType[*apierror.APIError](err); ok {
		godump.Dump(apiErr)
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
	if googErr, ok := errors.AsType[*googleapi.Error](err); ok {
		godump.Dump(googErr)
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
