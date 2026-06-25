// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"

	"github.com/immanent-tech/foragd/models"
)

// APIError wraps the error returned by a GCP API endpoint in a models.APIError.
func APIError(desc string, err error) error {
	if apiError, ok := errors.AsType[*googleapi.Error](err); ok {
		return models.NewAPIError(apiError.Code,
			fmt.Errorf("%s: %w", desc, apiError),
		)
	}
	return models.NewAPIError(
		http.StatusInternalServerError,
		fmt.Errorf("%s: %w", desc, err),
	)
}
