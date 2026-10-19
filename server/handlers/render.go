/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type pageMetadata struct {
	Title       pageTitle
	Description string
	Path        string
	ImagePath   string
	baseURL     *url.URL
}

func (m pageMetadata) CanonicalLink() string {
	return m.baseURL.JoinPath(m.Path).String()
}

func (m pageMetadata) OpengraphData() *opengraph.WebSite {
	return opengraph.NewWebSite(
		m.Title.String(),
		m.baseURL.Clone().JoinPath(m.Path).String(),
		m.Description,
		m.baseURL.Clone().JoinPath(m.ImagePath).String(),
	)
}

func (m pageMetadata) JSONLD() *schemaorg.WebPage {
	return schemaorg.NewWebPage(
		m.baseURL.Clone().JoinPath(m.Path).String(),
		m.Title.Summary,
		m.Title.Description,
		m.Description,
		"",
		"",
		"en",
		m.baseURL.String(),
		"",
		m.baseURL.Clone().JoinPath(m.ImagePath).String(),
		"",
		"",
	)
}

type pageServices struct {
	appCfg      AppConfig
	sessionMgr  SessionManager
	breadcrumbs Breadcrumbs
}

type pageTitle struct {
	Summary     string
	Description string
	Date        string
}

func (t pageTitle) String() string {
	if t.Summary == "" {
		return ""
	}
	var title strings.Builder
	title.WriteString(t.Summary)
	if t.Description != "" {
		title.WriteString(": ")
		title.WriteString(t.Description)
	}
	if t.Date != "" {
		title.WriteString(" (")
		title.WriteString(t.Date)
		title.WriteString(")")
	}
	title.WriteString(" | ")
	title.WriteString("Foragd")
	return title.String()
}

type Page struct {
	pageMetadata
	pageServices

	nav      templates.Navigation
	template templ.Component
}

var _ templates.Page = (*Page)(nil)

func (m *Manager) NewPage(path string, options ...PageOption) *Page {
	page := &Page{
		pageMetadata: pageMetadata{
			Path:      path,
			baseURL:   m.AppConfig.GetBaseURL().Clone(),
			ImagePath: "/images/logo-vertical-light.webp",
		},
		pageServices: m.NewPageServices(),
	}
	for option := range slices.Values(options) {
		option(page)
	}
	return page
}

// GetTitle returns the fully rendered title for the page.
func (p Page) GetTitle() string {
	return p.Title.String()
}

// GetDescription returns the metadata description of the page.
func (p Page) GetDescription() string {
	return p.pageMetadata.Description
}

// GetTemplate returns the [templ.Component] template for the page. This template will be wrapped for full page
// rendering.
func (p Page) GetTemplate() templ.Component {
	return p.template
}

func (p Page) FullResponse(res http.ResponseWriter, req *http.Request) {
	if p.template != nil {
		// Render a default full response.
		templ.Handler(templates.CreatePage(p.appCfg, p.sessionMgr, p)).ServeHTTP(res, req)
		return
	}
	slogctx.Warn(req.Context(), "FullResponse called but no template for page")
	res.WriteHeader(http.StatusNoContent)
}

func (p Page) PartialResponse(res http.ResponseWriter, req *http.Request) {
	// Use the page's own full response handler if defined.
	if p.template != nil {
		templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.Title(p.GetTitle())).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(p.nav, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
		templ.Handler(templates.Dock(p.nav, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
		return
	}
	slogctx.Warn(req.Context(), "PartialResponse called but no template for page")
	res.WriteHeader(http.StatusNoContent)
}

func (p Page) Handle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx, span := tracer.Start(req.Context(), "pageRender")
		defer span.End()

		switch {
		case !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req): // Non-HTMX or HistoryRestoreRequests render a full-page.
			p.FullResponse(res, req.WithContext(ctx))
		default: // HTMX request renders partial content.
			p.PartialResponse(res, req.WithContext(ctx))
		}
	}
}

// ToJsonLd returns the JSON-LD schema for the page as a [templ.Component].
func (p Page) ToJsonLd() templ.Component {
	if jsonLD := p.JSONLD(); jsonLD != nil {
		return jsonLD.ToJsonLd()
	}
	return nil
}

// ToMetaTags returns the OpenGraph data for the page as a [templ.Component].
func (p Page) ToMetaTags() templ.Component {
	if og := p.OpengraphData(); og != nil {
		return og.ToMetaTags()
	}
	return nil
}

// PageOption is a functional option applied to a page.
type PageOption func(*Page)

func WithSummary(summary string) PageOption {
	return func(p *Page) {
		p.Title.Summary = summary
	}
}

func WithDescription(description string) PageOption {
	return func(p *Page) {
		p.Title.Description = description
	}
}

func WithDate(date string) PageOption {
	return func(p *Page) {
		p.Title.Date = date
	}
}

func WithMetadataDescription(description string) PageOption {
	return func(p *Page) {
		p.pageMetadata.Description = description
	}
}

func WithNavigation(nav templates.Navigation) PageOption {
	return func(p *Page) {
		p.nav = nav
	}
}

func WithTemplate(template templ.Component) PageOption {
	return func(p *Page) {
		p.template = template
	}
}

// Partial represents a partial template. Partial templates do not render a full page.
type Partial struct {
	pageServices
	pageTitle

	fragments []string
	template  templ.Component
}

// NewPartial creates a new [Partial] that will render a partial template.
func (m *Manager) NewPartial(template templ.Component, options ...PartialOption) *Partial {
	partial := &Partial{
		pageServices: m.NewPageServices(),
		template:     template,
	}
	for option := range slices.Values(options) {
		option(partial)
	}
	return partial
}

func (p Partial) GetTitle() string {
	return p.pageTitle.String()
}

// Handle renders the [Partial] to the response. If the request is not HTMX-based, it returns
// [http.StatusNotAcceptable]. Otherwise the template is rendered (or the specified fragments of the template are
// rendered).
func (p Partial) Handle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx, span := tracer.Start(req.Context(), "partialRender")
		defer span.End()

		switch {
		case !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req): // Non-HTMX or HistoryRestoreRequests render a full-page.
			// If the request is not a HTMX request, return 406: Not Acceptable.
			slogctx.Warn(ctx, "Partial render requested but not a HTMX request.")
			res.WriteHeader(http.StatusNotAcceptable)
			return
		default: // HTMX request renders partial content.
			if len(p.fragments) > 0 {
				templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).
					ServeHTTP(res, req.WithContext(ctx))
			} else {
				templ.Handler(p.template).ServeHTTP(res, req.WithContext(ctx))
			}
			if title := p.GetTitle(); title != "" {
				templ.Handler(templates.Title(p.GetTitle())).ServeHTTP(res, req.WithContext(ctx))
			}
		}
	}
}

// PartialOption is a functional option applied to a partial.
type PartialOption func(*Partial)

// WithTitleUpdate option will cause the page title to update with the given [pageTitle].
func WithTitleUpdate(title pageTitle) PartialOption {
	return func(p *Partial) {
		p.pageTitle = title
	}
}

// WithFragments option will only render the given fragment from the template.
func WithFragments(fragment ...string) PartialOption {
	return func(p *Partial) {
		p.fragments = fragment
	}
}
