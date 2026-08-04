// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"
)

type Route = string

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var internalPageHandlerChain = alice.New(refreshOnHistoryRestore)

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

// refreshOnHistoryRestore will trigger a full client-side refresh on a HTMX history restore request.
func refreshOnHistoryRestore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if htmx.IsHistoryRestoreRequest(req) {
			slogctx.FromCtx(req.Context()).Debug("Is history restore")
			res.Header().Set(htmx.HeaderRefresh, "true")
		}
		next.ServeHTTP(res, req)
	})
}
