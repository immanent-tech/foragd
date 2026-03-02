// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package web

import (
	"embed"
	"log/slog"
	"slices"
	"strings"
)

//go:embed all:content
var StaticContentFS embed.FS

//go:embed all:assets/docs
var DocsFS embed.FS

func GetScriptFilenames() []string {
	scriptFiles := make([]string, 0, 2)
	dir, err := StaticContentFS.ReadDir("content")
	if err != nil {
		slog.Error("Read embedded static files failed.",
			slog.Any("error", err))
	}
	for file := range slices.Values(dir) {
		if strings.HasPrefix(file.Name(), "scripts") && !strings.HasSuffix(file.Name(), "map") {
			scriptFiles = append(scriptFiles, file.Name())
		}
	}
	return scriptFiles
}
