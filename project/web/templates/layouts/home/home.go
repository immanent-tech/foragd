// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

var (
	ErrHomePartialRender = errors.New("partial render of home failed")
	ErrHomeFullRender    = errors.New("full render of home failed")
)

const (
	ContentTarget = "#content"
)

// Element represents a simple Component that can be rendered on the page with
// its Show method.
type Element interface {
	Show(classes ...string) templ.Component
}

// Part represents a complex Component that requires different handling
// depending on whether the request is HTMX powered or not.
type Part interface {
	// PartialRender is called when a partial (i.e., HTMX) request is made.
	PartialRender() templ.Component
	// FullRender is called when a full (i.e., page reload or request) is made.
	FullRender() templ.Component
}

// LayoutOption is an option that can be applied to the home page layout.
type LayoutOption func(*LayoutProps)

// LayoutProps is the home page layout.
type LayoutProps struct {
	parts []Part
	title string
}

// Render will render the /home layout. This will either be a partial or full
// render depending on whether the request is htmx powered or not.
func (l *LayoutProps) Render(res http.ResponseWriter, req *http.Request) error {
	resp := htmx.NewResponse()

	content := make([]templ.Component, 0, len(l.parts))

	// Render parts.
	if htmx.IsHTMX(req) {
		for _, part := range l.parts {
			content = append(content, part.PartialRender())
		}
		if err := resp.RenderTempl(req.Context(), res, templ.Join(content...)); err != nil {
			return errors.Join(ErrHomePartialRender, err)
		}
	} else {
		content = append(content, commandModal())
		for _, part := range l.parts {
			content = append(content, part.FullRender())
		}
		// Render full page.
		fullPage := layouts.BuildPage(
			layouts.WithHeadOptions(l.title,
				layouts.WithPageDescription("Your home."),
				layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			),
			layouts.WithPageContent(content...),
		)
		if err := resp.RenderTempl(req.Context(), res, fullPage.Show()); err != nil {
			return errors.Join(ErrHomePartialRender, err)
		}
	}

	return nil
}

// WithPart adds a Part to the /home page.
func WithParts(parts ...Part) LayoutOption {
	return func(layout *LayoutProps) {
		layout.parts = parts
	}
}

// WithTitle will set a new page title.
func WithTitle(title string) LayoutOption {
	return func(layout *LayoutProps) {
		layout.title = title
	}
}

// BuildHomeLayout builds the home page layout from the given options.
func BuildLayout(options ...LayoutOption) *LayoutProps {
	layout := &LayoutProps{}

	for _, option := range options {
		option(layout)
	}

	return layout
}
