// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"log/slog"
	"os"
)

func main() {
	// Run your server.
	if err := runServer(); err != nil {
		slog.Error("Failed to start server!", slog.Any("error", err))
		os.Exit(1)
	}
}
