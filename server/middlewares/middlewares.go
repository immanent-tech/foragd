/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"net/url"

	"github.com/immanent-tech/go-base/config"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/server/middlewares")

type AppConfig interface {
	GetAppID() string
	GetAppVersion() string
	GetAppName() string
	GetAppEnvironment() config.Environment
	IsProduction() bool
	GetBaseURL() *url.URL
}
