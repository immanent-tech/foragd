// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package slots

import (
	"context"

	"github.com/a-h/templ"
)

type slotKey string

const (
	Header = "header"
)

// WithSlot stores a component in context under a named key.
func WithSlot(ctx context.Context, key string, c templ.Component) context.Context {
	return context.WithValue(ctx, slotKey(key), c)
}

// GetSlot retrieves a component from context by key. Returns nil if the Slot was not set.
func GetSlot(ctx context.Context, key string) templ.Component {
	c, _ := ctx.Value(slotKey(key)).(templ.Component)
	return c
}

// HasSlot reports whether a Slot was set in context.
func HasSlot(ctx context.Context, key string) bool {
	return GetSlot(ctx, key) != nil
}
