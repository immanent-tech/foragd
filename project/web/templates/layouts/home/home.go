// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
)

var (
	ErrHomePartialRender = errors.New("partial render of home failed")
	ErrHomeFullRender    = errors.New("full render of home failed")
)

var homePage = layouts.BuildPage("Go Feed Me - Home",
	layouts.WithPageDescription("Your home."),
	layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"))

const (
	AppBar LayoutPart = "appbar"
	Header LayoutPart = "header"
	Footer LayoutPart = "footer"
	Card   LayoutPart = "card"

	ContentTarget = "#content"
)

// LayoutPart defines a part of the home page, usually a component that will be
// an OOB swap on the page.
type LayoutPart string

type Content interface {
	Show(classes ...string) templ.Component
}

// LayoutOption is an option that can be applied to the home page layout.
type LayoutOption func(*LayoutProps)

// LayoutProps is the home page layout.
type LayoutProps struct {
	parts   map[LayoutPart]templ.Component
	content []Content
}

// WithPart adds a layout "part"; a component that will be OOB swapped into the
// page along with the main content.
func WithPart(name LayoutPart, part templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.parts[name] = part
	}
}

// WithContent is the content to be loaded into the page. If there is no
// content, a content placeholder will be loaded instead.
func WithContent(content ...Content) LayoutOption {
	return func(layout *LayoutProps) {
		layout.content = content
	}
}

// BuildHomeLayout builds the home page layout from the given options.
func BuildLayout(options ...LayoutOption) *LayoutProps {
	layout := &LayoutProps{
		parts: make(map[LayoutPart]templ.Component),
	}

	for _, option := range options {
		option(layout)
	}

	return layout
}

// Render will render the home page, choosing whether to do a full page load or
// partial depending on the HTMX headers.
func (layout *LayoutProps) Render(req *http.Request, res http.ResponseWriter) error {
	header := req.Header.Get(htmx.HeaderTarget)

	switch {
	case "#"+header == ContentTarget:
		return layout.PartialRender(req.Context(), res)
	case strings.HasPrefix(header, "feed_"):
		return layout.PartRender(req.Context(), res, Card)
	case header == "":
		return layout.FullRender(req.Context(), res)
	default:
		return errors.New("invalid render target")
	}
}

// FullRender is a full home page render with all parts/components that are set.
func (layout *LayoutProps) FullRender(ctx context.Context, res http.ResponseWriter) error {
	resp := htmx.NewResponse()

	WithPart(AppBar, appbar.AppBar().Show())(layout)

	layouts.WithPageContent(layout.showFullLayout())(homePage)

	if err := resp.RenderTempl(ctx, res, homePage.Show()); err != nil {
		return errors.Join(ErrHomeFullRender, err)
	}

	return nil
}

// PartialRender does a partial render, targeting the main content and relying
// on OOB swaps for the optional parts.
func (layout *LayoutProps) PartialRender(ctx context.Context, res http.ResponseWriter) error {
	var warnings error

	resp := htmx.NewResponse()

	if err := resp.RenderTempl(ctx, res, layout.ShowContent()); err != nil {
		return errors.Join(ErrHomePartialRender, fmt.Errorf("could not render content: %w", err))
	}

	for name, part := range layout.parts {
		if err := resp.RenderTempl(ctx, res, part); err != nil {
			warnings = errors.Join(warnings, fmt.Errorf("could not render %s: %w", name, err))
		}
	}

	if warnings != nil {
		return errors.Join(ErrHomePartialRender, warnings)
	}

	return nil
}

// PartRender renders just the specified part, usually as an OOB swap.
func (layout *LayoutProps) PartRender(ctx context.Context, res http.ResponseWriter, part LayoutPart) error {
	resp := htmx.NewResponse()

	component, found := layout.parts[part]
	if !found {
		return errors.Join(ErrHomePartialRender, fmt.Errorf("missing part %s", part))
	}

	if err := resp.RenderTempl(ctx, res, component); err != nil {
		return errors.Join(ErrHomePartialRender, fmt.Errorf("could not render part %s: %w", part, err))
	}

	return nil
}
