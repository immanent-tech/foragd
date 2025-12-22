// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/angelofallars/htmx-go"
	"github.com/rs/cors"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

// CORS contains values for various CORS settings derived from the environment.
type CORS struct {
	AllowedOrigins  []string `koanf:"allowedorigins"`
	MaxAge          int      `koanf:"maxage"`
	RequestHeaders  []string `koanf:"requestheaders"`
	ResponseHeaders []string `koanf:"responseheaders"`
}

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

var corsOptions cors.Options

var loadCORS = sync.OnceValue(func() error {
	corsSettings, err := config.Load[CORS](config.ConfigEnvPrefix + "CORS_")
	if err != nil {
		return fmt.Errorf("load cors config: %w", err)
	}

	corsOptions = cors.Options{
		AllowCredentials:    true,
		MaxAge:              corsSettings.MaxAge,
		AllowPrivateNetwork: true,
		OptionsPassthrough:  true,
		AllowedHeaders: append(
			[]string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			HTMXRequestHeaders...,
		),
		ExposedHeaders: append(
			[]string{"Link", "Accept-CH"},
			HTMXResponseHeaders...,
		),
		AllowedMethods: []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodOptions},
		AllowedOrigins: corsSettings.AllowedOrigins,
	}

	return nil
})

// SetupCORS handles adding the appropriate headers for CORS to the request.
func SetupCORS(next http.Handler) http.Handler {
	if err := loadCORS(); err != nil {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			slogctx.FromCtx(req.Context()).Error("Cannot load CORS config.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
		})
	}
	return cors.New(corsOptions).Handler(next)
}
