// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/display/text"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"
	"github.com/joshuar/go-templ-daisyui/navigation/breadcrumbs"
	"github.com/joshuar/go-templ-daisyui/navigation/link"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"
	"github.com/joshuar/go-templ-daisyui/navigation/navbar"

	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
)

var (
	ErrHomePartialRender = errors.New("partial render of home failed")
	ErrHomeFullRender    = errors.New("full render of home failed")
)

// LayoutOption is an option that can be applied to the home page layout.
type LayoutOption templates.Option[*LayoutProps]

// LayoutProps is the home page layout.
type LayoutProps struct {
	SideBar       *menu.Props
	AppBar        *navbar.Props
	ContentHeader templ.Component
	Breadcrumbs   *breadcrumbs.Props
	Content       []*templates.Component
	ContentFooter templ.Component
}

// WithContent option defines the content to display on the page.
func WithContent(content ...*templates.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.Content = content
	}
}

func WithHeader(header templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.ContentHeader = header
	}
}

// WithFooter option controls the options for the home page footer.
func WithFooter(footer templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.ContentFooter = footer
	}
}

// WithBreadCrumbs option defines the breadcrumbs to display on the home page.
func WithBreadCrumbs(crumbs ...templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		allCrumbs := make([]templ.Component, 0, len(crumbs)+1)
		// Always add a link to "/home as the first crumb.
		allCrumbs = append(allCrumbs,
			button.Build(
				button.WithSize(size.SM),
				button.AsShape(button.Square, false),
				button.WithThemeColor(color.Ghost, false),
				button.WithContent(
					icon.Build("fa-house"),
				),
			).Show(),
		)
		// Append other crumbs.
		allCrumbs = append(allCrumbs, crumbs...)
		layout.Breadcrumbs = breadcrumbs.Build(
			breadcrumbs.WithCrumbs(allCrumbs...),
		)
	}
}

// BuildCrumb creates a new breadcrumb.
func BuildCrumb(name string, weight text.Weight, attributes templ.Attributes) templ.Component {
	return link.Build(link.WithContent(text.Build(name,
		text.WithTextWeight(weight),
		text.WithTextSize(text.SM))),
		link.WithUnderlineOnHover(),
		link.WithExtraAttributes(attributes),
	).Show()
}

// WithSideBar option defines the layout of the drawer side rail on the home page.
func WithSideBar(options ...menu.Option) LayoutOption {
	return func(layout *LayoutProps) {
		layout.SideBar = menu.Build(options...)
	}
}

// BuildHomeLayout builds the home page layout from the given options.
func BuildLayout(options ...LayoutOption) *LayoutProps {
	layout := &LayoutProps{
		// TODO: implement appbar as option.
		AppBar:        appbar.AppBar(),
		ContentFooter: Footer(),
		// ContentHeader: Header(),
	}

	for _, option := range options {
		option(layout)
	}

	return layout
}

func (layout *LayoutProps) Render(ctx context.Context, res http.ResponseWriter, resp htmx.Response, isHTMX bool) error {
	if !isHTMX {
		return layout.FullRender(ctx, res, resp)
	}

	return layout.PartialRender(ctx, res, resp)
}

func (layout *LayoutProps) FullRender(ctx context.Context, res http.ResponseWriter, resp htmx.Response) error {
	page := layouts.BuildPage("Go Feed Me - Home",
		layouts.WithPageDescription("Your home."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layout.homeDrawer())).Show()
	if err := resp.RenderTempl(ctx, res, page); err != nil {
		return errors.Join(ErrHomeFullRender, err)
	}

	return nil
}

func (layout *LayoutProps) PartialRender(ctx context.Context, res http.ResponseWriter, resp htmx.Response) error {
	if err := resp.RenderTempl(ctx, res, layout.ShowContent()); err != nil {
		return errors.Join(ErrHomePartialRender, err)
	}
	// OOB update breadcrumbs.
	if err := resp.RenderTempl(ctx, res, layout.ShowBreadcrumbs()); err != nil {
		return errors.Join(ErrHomePartialRender, err)
	}
	// OOB update header.
	if layout.ContentHeader != nil {
		if err := resp.RenderTempl(ctx, res, layout.ContentHeader); err != nil {
			return errors.Join(ErrHomePartialRender, err)
		}
	}
	// OOB update footer.
	if layout.ContentFooter != nil {
		if err := resp.RenderTempl(ctx, res, layout.ContentFooter); err != nil {
			return errors.Join(ErrHomePartialRender, err)
		}
	}
	// OOB update drawer sidebar.
	if err := resp.RenderTempl(ctx, res, layout.SideBar.Show()); err != nil {
		return errors.Join(ErrHomePartialRender, err)
	}

	return nil
}
