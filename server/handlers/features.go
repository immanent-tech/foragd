// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/web/templates"
)

type Features struct {
	pageMetadata
	svc pageServices
}

func (m *Manager) HandleFeatures() http.HandlerFunc {
	return RenderExternalPage(&Features{
		Title: templates.PageTitle{
			Summary:     "Features",
			Description: "RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
		Description: "Discover Foragd's features: subscribe to any RSS feed, YouTube channel, newsletter or subreddit, organise with smart folders, and read distraction-free. No ads, no algorithms.",
		Path:        "/features",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
		svc:         m.NewPageServices(),
	})
}

func (p *Features) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		p.svc.appCfg,
		p.svc.sessionMgr,
		templates.Features(),
		templates.WithPageTitle(p.Title),
		templates.WithPageDescription(p.Description),
		templates.WithCanonicalLink(p.CanonicalLink()),
		templates.WithOpenGraphMetadata(p.OpengraphData()),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.baseURL),
			p.JSONLD(),
		),
	)).ServeHTTP(res, req)
}

type FeaturesCollect struct {
	pageMetadata
	svc pageServices
}

func (m *Manager) HandleFeaturesCollect() http.HandlerFunc {
	return RenderExternalPage(&FeaturesCollect{
		Title: templates.PageTitle{
			Summary:     "Collect",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
		Description: "Discover Foragd's features focused around collection: add any website, blog, YouTube channel, Reddit subreddit, or email newsletter easily.",
		Path:        "/features/collect",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
		svc:         m.NewPageServices(),
	})
}

func (p *FeaturesCollect) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		p.svc.appCfg,
		p.svc.sessionMgr,
		templates.FeaturesPageCollect(p.baseURL),
		templates.WithPageTitle(p.Title),
		templates.WithPageDescription(p.Description),
		templates.WithCanonicalLink(p.CanonicalLink()),
		templates.WithOpenGraphMetadata(p.OpengraphData()),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.baseURL),
			p.JSONLD(),
		),
	)).ServeHTTP(res, req)
}

type FeaturesCurate struct {
	pageMetadata
	svc pageServices
}

func (m *Manager) HandleFeaturesCurate() http.HandlerFunc {
	return RenderExternalPage(&FeaturesCurate{
		Title: templates.PageTitle{
			Summary:     "Curate",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
		Description: "Discover Foragd's features focused around curation: group subscriptions, save searches as subscriptions and filter articles easily.",
		Path:        "/features/curate",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
		svc:         m.NewPageServices(),
	})
}

func (p *FeaturesCurate) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		p.svc.appCfg,
		p.svc.sessionMgr,
		templates.FeaturesPageCurate(p.baseURL),
		templates.WithPageTitle(p.Title),
		templates.WithPageDescription(p.Description),
		templates.WithCanonicalLink(p.CanonicalLink()),
		templates.WithOpenGraphMetadata(p.OpengraphData()),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.baseURL),
			p.JSONLD(),
		),
	)).ServeHTTP(res, req)
}

type FeaturesConsume struct {
	pageMetadata
	svc pageServices
}

func (m *Manager) HandleFeaturesConsume() http.HandlerFunc {
	return RenderExternalPage(&FeaturesConsume{
		Title: templates.PageTitle{
			Summary:     "Consume",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
		Description: "Discover Foragd's features focused around consumption: fetch content directly from the source, customise the UI and more.",
		Path:        "features/consume",
		ImagePath:   "/content/logo-vertical-light.webp",
		baseURL:     m.AppConfig.GetBaseURL(),
		svc:         m.NewPageServices(),
	})
}

func (p *FeaturesConsume) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.CreatePage(
		p.svc.appCfg,
		p.svc.sessionMgr,
		templates.FeaturesPageConsume(p.baseURL),
		templates.WithPageTitle(p.Title),
		templates.WithPageDescription(p.Description),
		templates.WithCanonicalLink(p.CanonicalLink()),
		templates.WithOpenGraphMetadata(p.OpengraphData()),
		templates.WithJSONLDSchema(
			generateSiteJSONLD(p.baseURL),
			p.JSONLD(),
		),
	)).ServeHTTP(res, req)
}
