/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("github.com/immanent-tech/foragd/server/middlewares")
