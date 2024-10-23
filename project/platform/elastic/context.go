// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package elastic

import (
	"context"
)

type contextKey string

const (
	clientContextKey contextKey = "client"
)

func ContextSetClient(ctx context.Context, client *Client) context.Context {
	newCtx := context.WithValue(ctx, clientContextKey, client)

	return newCtx
}

func ContextGetClient(ctx context.Context) (*Client, bool) {
	state, ok := ctx.Value(clientContextKey).(*Client)
	if !ok {
		return nil, false
	}

	return state, true
}
