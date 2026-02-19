// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"

	"github.com/justinas/alice"
	"github.com/riandyrn/otelchi"

	otelchimetric "github.com/riandyrn/otelchi/metric"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/server/otel"
)

// Otel is a middleware to configure open telemetry for the server.
func Otel(next http.Handler) http.Handler {
	return alice.New(
		otelchi.Middleware(config.AppName),
		otelchimetric.NewRequestDurationMillis(otel.MeterConfig),
		otelchimetric.NewRequestInFlight(otel.MeterConfig),
		otelchimetric.NewResponseSizeBytes(otel.MeterConfig),
	).Then(next)
}
