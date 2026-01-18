//go:build mage
// +build mage

package main

import (
	"os/exec"
	// mg contains helpful utility functions, like Deps
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

func Run() error {
	cmd := exec.Command(
		"bunx",
		"dotenv-cli",
		"-e",
		".env.development",
		"--",
		"go",
		"run",
		`-ldflags=\"-X github.com/immanent-tech/foragd/config.Version=$(git describe --tags --always --long --dirty)\"`,
		"main.go",
		"--log-level=debug",
		"--no-log-file",
		`--profile='heapprofile=deployments/scheduler-heap.prof;cpuprofile=deployments/scheduler-cpu.prof;webui=true'`,
		"run",
		"--help",
	)
	return cmd.Run()
}
