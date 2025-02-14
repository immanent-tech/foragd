// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-templ-daisyui/display/text"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/navigation/breadcrumbs"
	"github.com/joshuar/go-templ-daisyui/navigation/link"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"
	"github.com/joshuar/go-templ-daisyui/navigation/navbar"

	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
)

var (
	ErrHomePartialRender = errors.New("partial render of home failed")
	ErrHomeFullRender    = errors.New("full render of home failed")
)

// LayoutOption is an option that can be applied to the home page layout.
type LayoutOption templates.Option[*LayoutProps]

// LayoutProps is the home page layout.
type LayoutProps struct {
	SideBar     *menu.Props
	AppBar      *navbar.Props
	Breadcrumbs *breadcrumbs.Props
	Content     []*templates.Component
	Footer      templ.Component
}

func WithContent(content ...*templates.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.Content = content
	}
}

func WithFooter(footer templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		layout.Footer = footer
	}
}

func WithBreadCrumbs(crumbs ...templ.Component) LayoutOption {
	return func(layout *LayoutProps) {
		allCrumbs := make([]templ.Component, 0, len(crumbs)+1)
		// Always add a link to "/home as the first crumb.
		allCrumbs = append(allCrumbs,
			BuildCrumb("Home", text.Normal, templ.Attributes{
				"hx-get":      "/home",
				"hx-push-url": "true",
			}))
		// Append other crumbs.
		allCrumbs = append(allCrumbs, crumbs...)
		layout.Breadcrumbs = breadcrumbs.Build(
			breadcrumbs.WithCrumbs(allCrumbs...),
		)
	}
}

func BuildCrumb(name string, weight text.Weight, attributes templ.Attributes) templ.Component {
	return link.Build(link.WithContent(text.Build(name,
		text.WithTextWeight(weight),
		text.WithTextSize(text.SM))),
		link.WithExtraAttributes(attributes),
	).Show()
}

func WithSideBar(options ...menu.Option) LayoutOption {
	return func(layout *LayoutProps) {
		layout.SideBar = menu.Build(options...)
	}
}

// BuildHomeLayout builds the home page layout from the given options.
func BuildLayout(options ...LayoutOption) *LayoutProps {
	layout := &LayoutProps{
		// TODO: implement appbar as option.
		AppBar: navbar.Build(navbar.WithID("content_app_bar"),
			navbar.WithBaseColor(color.Base200),
			navbar.NavBarStart(appBarTopLeft()),
			navbar.NavBarEnd(appBarTopRight()),
			navbar.NavBarCenter(appBarTopCenter())),
		Footer: Footer(),
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
	// OOB update footer.
	if layout.Footer != nil {
		if err := resp.RenderTempl(ctx, res, layout.Footer); err != nil {
			return errors.Join(ErrHomePartialRender, err)
		}
	}
	// OOB update drawer sidebar.
	if err := resp.RenderTempl(ctx, res, layout.SideBar.Show()); err != nil {
		return errors.Join(ErrHomePartialRender, err)
	}

	return nil
}
