// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/immanent-tech/foragd/server"
)

// ServeCmd defines the `server` command for running the server.
type ServeCmd struct{}

// Run performs setup and execution for the server command.
func (r *ServeCmd) Run(opts *Arguments) error {
	if err := server.Start(opts.Logger); err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}
	return nil
}
