// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

//go:generate go tool oapi-codegen -config models-cfg.yaml models.yaml
//go:generate go tool oapi-codegen -config api-cfg.yaml api.yaml
//go:generate go tool oapi-codegen -config jobs-cfg.yaml jobs.yaml
