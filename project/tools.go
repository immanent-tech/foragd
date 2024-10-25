// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:build tools
// +build tools

package main

import (
	_ "github.com/a-h/templ/cmd/templ"
	_ "github.com/air-verse/air"
	_ "github.com/davecgh/go-spew/spew"
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/yassinebenaid/godump"
	_ "golang.org/x/tools/cmd/stringer"
)
