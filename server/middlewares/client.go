// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	slogctx "github.com/veqryn/slog-context"
)

// ClientContext is a string that represents which type of client is accessing the app.
type ClientContext string

const (
	ClientWeb ClientContext = "web"
	ClientTWA ClientContext = "twa"
	ClientPWA ClientContext = "pwa"
)

// DetectClient detects the type of client accessing the app.
func DetectClient(req *http.Request) ClientContext {
	// Explicit header — most reliable
	if req.Header.Get("X-TWA-Client") != "" {
		return ClientTWA
	}

	// TWA WebView UA contains package ID and wv token
	if ua := req.Header.Get("User-Agent"); strings.Contains(ua, "app.foragd.twa") && strings.Contains(ua, "wv") {
		return ClientTWA
	}

	// Standalone PWA (installed, not TWA)
	if req.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		req.Header.Get("Sec-Fetch-Site") == "none" {
		return ClientPWA
	}

	return ClientWeb
}

// SetClient is a middleware that detects and sets a client variable in the context.
func SetClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		client := DetectClient(req)
		ctx := context.WithValue(req.Context(), "client", client)
		ctx = slogctx.With(ctx, slog.String("client", string(client)))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
