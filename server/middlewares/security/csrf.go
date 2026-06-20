// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package security

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"
)

func PreventCSRF(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()

	cop.AddInsecureBypassPattern("/checkout/webhooks")
	cop.AddInsecureBypassPattern("/mail/webhooks")

	cop.SetDenyHandler(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusBadRequest)
		res.Write([]byte("CSRF check failed"))
	}))

	return cop.Handler(next)
}

// CSRFError handles CSRF error conditions. It will log details about the request then show an error page to the user.
func CSRFError() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		params := make(map[string]string)
		if chi.RouteContext(req.Context()) != nil {
			if len(chi.RouteContext(req.Context()).URLParams.Keys) > 0 {
				for i, k := range chi.RouteContext(req.Context()).URLParams.Keys {
					params[k] = chi.RouteContext(req.Context()).URLParams.Values[i]
				}
			}
		}
		slogctx.FromCtx(req.Context()).Error("CSRF check failed",
			slog.String("method", req.Method),
			slog.String("host", req.Host),
			slog.String("path", req.URL.Path),
			slog.String("query", req.URL.RawQuery),
			slog.Any("params", params),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("ip", req.RemoteAddr),
			slog.String("referer", req.Referer()),
			slog.String(slogchi.RequestIDKey, middleware.GetReqID(req.Context())),
		)
		http.Error(res, "CSRF Failed", http.StatusBadRequest)
	}
}
