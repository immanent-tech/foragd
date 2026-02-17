// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"fmt"

	reverseproxy "github.com/immanent-tech/foragd/reverseproxy/go"
)

// ReverseProxyCmd defines the `reverseproxy` command, for running the reverse proxy.
type ReverseProxyCmd struct {
	Run RunReverseProxyCmd `cmd:"run" help:"Run reverse proxy."`
}

type RunReverseProxyCmd struct{}

// Run contains logic for setup and execution of the reverse proxy.
func (c *RunReverseProxyCmd) Run(opts *Arguments) error {
	if err := reverseproxy.Start(opts.Logger); err != nil {
		return fmt.Errorf("could not run reverse proxy: %w", err)
	}
	return nil
}
