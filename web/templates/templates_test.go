/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"
)

// render renders a component and returns a parsed DOM for assertions.
func render(t *testing.T, ctx context.Context, c templ.Component) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

// renderFragment renders a fragment and returns a parsed DOM for assertions.
func renderFragment(t *testing.T, ctx context.Context, c templ.Component, id any) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := templ.RenderFragments(ctx, &buf, c, id); err != nil {
		t.Fatalf("render fragment: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}
