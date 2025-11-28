// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/realclientip/realclientip-go"
	slogctx "github.com/veqryn/slog-context"
)

const (
	clientIPHeader = "X-Forwarded-For"
)

// RateLimiter holds options for controlling a rate limiter middleware.
type RateLimiter struct {
	strategy realclientip.RightmostNonPrivateStrategy
	limiter  *limiter.Limiter
}

// NewRateLimiter initialises data for a rate limiter middleware.
func NewRateLimiter() RateLimiter {
	// Set up rate-limiting.
	strategy, err := realclientip.NewRightmostNonPrivateStrategy(clientIPHeader)
	if err != nil {
		panic("realclientip.NewRightmostNonPrivateStrategy returned error (bad input)")
	}
	lmt := tollbooth.NewLimiter(5, nil)
	lmt.SetIPLookup(limiter.IPLookup{
		Name:           clientIPHeader,
		IndexFromRight: 0,
	})
	return RateLimiter{
		strategy: strategy,
		limiter:  lmt,
	}
}

// RateLimit middleware will try to rate limit incoming requests with a pre-defined strategy.
func RateLimit(ratelimiter RateLimiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			env := os.Getenv("FORAGD_ENVIRONMENT")
			// Ignore rate-limiting in development environment or for health probes in GCP.
			if env == "development" || slices.Contains([]string{"/livenessProbe"}, req.URL.Path) {
				next.ServeHTTP(res, req)
				return
			}
			// Ignore rate-limiting from self.
			if req.Host == "foragd.app" {
				next.ServeHTTP(res, req)
				return
			}
			// Find the client IP.
			clientIP := ratelimiter.strategy.ClientIP(req.Header, req.RemoteAddr)
			if clientIP == "" {
				slogctx.FromCtx(req.Context()).Error("Unable to determine client IP.")
				http.Error(res, "I don't know who you are", http.StatusForbidden)
				return
			}
			// We don't want to include the zone in our limiter key
			clientIP, _ = realclientip.SplitHostZone(clientIP)

			httpErr := tollbooth.LimitByKeys(ratelimiter.limiter, []string{clientIP})
			if httpErr != nil {
				slogctx.FromCtx(req.Context()).Warn("Request rate-limited.",
					slog.String("error", httpErr.Message),
					slog.Int("code", httpErr.StatusCode),
				)
				http.Error(res, httpErr.Message, httpErr.StatusCode)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}
