/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"net/http"

	"github.com/justinas/alice"
	"github.com/riandyrn/otelchi"

	otelchimetric "github.com/riandyrn/otelchi/metric"

	"github.com/immanent-tech/foragd/server/otel"
)

// Otel is a middleware to configure open telemetry for the server.
func Otel(next http.Handler) http.Handler {
	if otel.IsEnabled() {
		return alice.New(
			otelchi.Middleware("foragd"),
			otelchimetric.NewServerRequestDuration(otel.MeterConfig),
			otelchimetric.NewServerActiveRequests(otel.MeterConfig),
			otelchimetric.NewServerResponseBodySize(otel.MeterConfig),
		).Then(next)
	}
	return next
}
