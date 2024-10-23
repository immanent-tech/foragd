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
