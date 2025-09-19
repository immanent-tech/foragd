// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/realclientip/realclientip-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

func RateLimiter(strat realclientip.RightmostNonPrivateStrategy, lmt *limiter.Limiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if config.Environment() == "development" {
				next.ServeHTTP(res, req)
				return
			}
			// Find the client IP.
			clientIP := strat.ClientIP(req.Header, req.RemoteAddr)
			if clientIP == "" {
				slogctx.FromCtx(req.Context()).Error("Unable to determine client IP.")
				http.Error(res, "I don't know who you are", http.StatusForbidden)
				return
			}
			// We don't want to include the zone in our limiter key
			clientIP, _ = realclientip.SplitHostZone(clientIP)

			if httpErr := tollbooth.LimitByKeys(lmt, []string{clientIP}); httpErr != nil {
				http.Error(res, httpErr.Message, httpErr.StatusCode)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}
