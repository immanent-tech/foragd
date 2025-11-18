// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/rs/cors"
)

const (
	// CORSMaxAge sets the maximum value not ignored by any of major browsers.
	CORSMaxAge = 300
)

// HTMXRequestHeaders contains all valid HTMX request headers.
//
// https://htmx.org/reference/#request_headers
var HTMXRequestHeaders = []string{
	htmx.HeaderBoosted,
	htmx.HeaderCurrentURL,
	htmx.HeaderHistoryRestoreRequest,
	htmx.HeaderPrompt,
	htmx.HeaderRequest,
	htmx.HeaderTarget,
	htmx.HeaderTriggerName,
	htmx.HeaderTrigger,
}

// HTMXResponseHeaders contains all valid HTMX response headers.
//
// https://htmx.org/reference/#response_headers
var HTMXResponseHeaders = []string{
	htmx.HeaderLocation,
	htmx.HeaderPushURL,
	htmx.HeaderRedirect,
	htmx.HeaderRefresh,
	htmx.HeaderReplaceUrl,
	htmx.HeaderReswap,
	htmx.HeaderRetarget,
	htmx.HeaderReselect,
	htmx.HeaderTriggerAfterSettle,
	htmx.HeaderTriggerAfterSwap,
	htmx.HeaderTrigger,
}

// SetupCORS handles adding the appropriate headers for CORS to the request.
func SetupCORS() func(next http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowCredentials:    true,
		MaxAge:              CORSMaxAge,
		AllowPrivateNetwork: true,
		OptionsPassthrough:  true,
		AllowedHeaders: append(
			[]string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			HTMXRequestHeaders...,
		),
		ExposedHeaders: append(
			[]string{"Link"},
			HTMXResponseHeaders...,
		),
	}).Handler
}
