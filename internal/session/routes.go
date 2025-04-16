// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
)

// GetRouteState fetches the last used state (i.e., all query options) for the
// given route from the user session. If this data cannot be retrieved, the
// route will be returned as-is.
func GetRouteState(ctx context.Context, route string) string {
	path := mgr.GetString(ctx, route)
	if path == "" {
		return route
	}

	return path
}

// SetRouteState stores the last used state (i.e., all query options) for the
// given route in the user session.
func SetRouteState(ctx context.Context, route, path string) {
	mgr.Put(ctx, route, path)
}
