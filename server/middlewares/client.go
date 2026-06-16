// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

// DetectClient detects the type of client accessing the app.
func DetectClient(req *http.Request) models.ClientType {
	// Explicit header — most reliable
	if req.Header.Get("X-Twa-Client") != "" {
		return models.ClientTypeTwa
	}

	// TWA WebView UA contains package ID and wv token
	if ua := req.Header.Get("User-Agent"); strings.Contains(ua, "app.foragd.twa") && strings.Contains(ua, "wv") {
		return models.ClientTypeTwa
	}

	// Standalone PWA (installed, not TWA)
	if req.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		req.Header.Get("Sec-Fetch-Site") == "none" {
		return models.ClientTypePwa
	}

	return models.ClientTypeWeb
}

// SetClient is a middleware that detects and sets a client variable in the context.
func SetClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		client := DetectClient(req)
		ctx := models.ClientTypeToCtx(req.Context(), client)
		ctx = slogctx.With(ctx, slog.String("client", string(client)))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
