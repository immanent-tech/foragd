// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"errors"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
)

var ErrHTMXRequired = errors.New("htmx is required for request")

func RequireHtmx(res http.ResponseWriter, req *http.Request) error {
	if !htmx.IsHTMX(req) {
		logging.FromContext(req.Context()).Error("Request was not made by htmx.")
		http.Error(res, "Invalid request", http.StatusBadRequest)

		return ErrHTMXRequired
	}

	return nil
}
