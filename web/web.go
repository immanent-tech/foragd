// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package web

import "embed"

//go:embed all:content
var StaticContentFS embed.FS

//go:embed all:assets/docs
var DocsFS embed.FS
