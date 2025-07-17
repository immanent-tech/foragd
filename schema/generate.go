// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package schemas contains the OpenAPI schema definitions for the application.
package schemas

//go:generate go tool oapi-codegen -config models-cfg.yaml models.yaml
//go:generate go tool oapi-codegen -config api-cfg.yaml api.yaml
//go:generate go tool oapi-codegen -config scheduler-cfg.yaml scheduler.yaml
