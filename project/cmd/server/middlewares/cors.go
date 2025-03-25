// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/rs/cors"
)

const (
	CORSMaxAge = 300 // Maximum value not ignored by any of major browsers.
)

func CORS(env string) func(next http.Handler) http.Handler {
	options := cors.Options{
		AllowCredentials:    true,
		MaxAge:              CORSMaxAge,
		AllowPrivateNetwork: true,
		OptionsPassthrough:  true,
	}

	if env == "development" {
		// options.Debug = true
		options.AllowedOrigins = []string{"*"}
	}

	options.AllowedHeaders = []string{
		"Accept", "Authorization", "Content-Type", "X-CSRF-Token",
		htmx.HeaderBoosted,
		htmx.HeaderCurrentURL,
		htmx.HeaderHistoryRestoreRequest,
		htmx.HeaderPrompt,
		htmx.HeaderRequest,
		htmx.HeaderTarget,
		htmx.HeaderTriggerName,
		htmx.HeaderTrigger,
	}

	options.ExposedHeaders = []string{
		"Link",
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

	corsH := cors.New(options)

	return corsH.Handler
}
