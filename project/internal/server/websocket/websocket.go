// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package websocket

import "github.com/lxzan/gws"

func NewWebsocketServer(handler gws.Event) *gws.Upgrader {
	return gws.NewUpgrader(handler, &gws.ServerOption{
		ParallelEnabled:   true,                                 // Parallel message processing
		Recovery:          gws.Recovery,                         // Exception recovery
		PermessageDeflate: gws.PermessageDeflate{Enabled: true}, // Enable compression
	})
}
