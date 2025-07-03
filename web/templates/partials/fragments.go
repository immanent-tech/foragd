// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"context"
	"io"
	"slices"

	"github.com/a-h/templ"
)

type fragmentComponent struct {
	name string
}

func (f fragmentComponent) Render(ctx context.Context, w io.Writer) error {
	// Get children.
	children := templ.GetChildren(ctx)
	ctx = templ.ClearChildren(ctx)
	if children == nil {
		return nil
	}
	// Render children.
	fw, ok := GetFragmentWriter(ctx, f.name)
	if ok {
		w = fw
	}
	return children.Render(ctx, w)
}

type (
	fragmentContextKey   struct{}
	fragmentContextValue struct {
		w     io.Writer
		names []string
	}
)

func WithFragment(ctx context.Context, w io.Writer, names ...string) context.Context {
	return context.WithValue(ctx, fragmentContextKey{}, fragmentContextValue{w: w, names: names})
}

func GetFragmentWriter(ctx context.Context, name string) (w io.Writer, ok bool) {
	val, ok := ctx.Value(fragmentContextKey{}).(fragmentContextValue)
	if !ok {
		return nil, false
	}
	if len(val.names) > 0 && !slices.Contains(val.names, name) {
		return nil, false
	}
	return val.w, true
}

func Fragment(name string) templ.Component {
	return fragmentComponent{name: name}
}
