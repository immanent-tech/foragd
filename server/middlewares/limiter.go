// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/realclientip/realclientip-go"
	slogctx "github.com/veqryn/slog-context"
)

// RateLimiter holds options for controlling a rate limiter middleware.
type RateLimiter struct {
	strategy realclientip.RightmostNonPrivateStrategy
	limiter  *limiter.Limiter
}

// NewRateLimiter initialises data for a rate limiter middleware.
func NewRateLimiter() RateLimiter {
	// Set up rate-limiting.
	strategy, err := realclientip.NewRightmostNonPrivateStrategy("X-Forwarded-For")
	if err != nil {
		panic("realclientip.NewRightmostNonPrivateStrategy returned error (bad input)")
	}
	limiter := tollbooth.NewLimiter(1, nil)
	return RateLimiter{
		strategy: strategy,
		limiter:  limiter,
	}
}

// RateLimit middleware will try to rate limit incoming requests with a pre-defined strategy.
func RateLimit(ratelimiter RateLimiter, env string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if env == "development" {
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
				http.Error(res, httpErr.Message, httpErr.StatusCode)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}
