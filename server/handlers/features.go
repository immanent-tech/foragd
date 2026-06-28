// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/web/templates"
)

type Features struct {
	title templates.PageTitle
}

func HandleFeatures() http.HandlerFunc {
	return RenderExternalPage(&Features{
		title: templates.PageTitle{
			Summary:     "Features",
			Description: "RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
	})
}

func (p *Features) FullResponse(res http.ResponseWriter, req *http.Request) {
	description := "Discover Foragd's features: subscribe to any RSS feed, YouTube channel, newsletter or subreddit, organise with smart folders, and read distraction-free. No ads, no algorithms."
	canonicalLink := config.GetBaseURL() + req.URL.String()
	templ.Handler(templates.CreatePage(templates.Features(),
		templates.WithPageTitle(p.title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(canonicalLink),
		templates.WithOpenGraphMetadata(opengraph.New(
			p.title.String(),
			"website",
			canonicalLink,
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
		)),
	)).ServeHTTP(res, req)
}

type FeaturesCollect struct {
	title templates.PageTitle
}

func HandleFeaturesCollect() http.HandlerFunc {
	return RenderExternalPage(&FeaturesCollect{
		title: templates.PageTitle{
			Summary:     "Collect",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
	})
}

func (p *FeaturesCollect) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Collect | Foragd Features | RSS Reader, Newsletter Aggregator & Feed Organiser"
	description := "Discover Foragd's features focused around collection: add any website, blog, YouTube channel, Reddit subreddit, or email newsletter easily."
	canonicalLink := config.GetBaseURL() + req.URL.String()
	templ.Handler(templates.CreatePage(templates.FeaturesPageCollect(),
		templates.WithPageTitle(p.title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(canonicalLink),
		templates.WithOpenGraphMetadata(opengraph.New(
			title,
			"website",
			canonicalLink,
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
		)),
	)).ServeHTTP(res, req)
}

type FeaturesCurate struct {
	title templates.PageTitle
}

func HandleFeaturesCurate() http.HandlerFunc {
	return RenderExternalPage(&FeaturesCurate{
		title: templates.PageTitle{
			Summary:     "Curate",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
	})
}

func (p *FeaturesCurate) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Curate | Foragd Features | RSS Reader, Newsletter Aggregator & Feed Organiser"
	description := "Discover Foragd's features focused around curation: group subscriptions, save searches as subscriptions and filter articles easily."
	canonicalLink := config.GetBaseURL() + req.URL.String()
	templ.Handler(templates.CreatePage(templates.FeaturesPageCurate(),
		templates.WithPageTitle(p.title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(canonicalLink),
		templates.WithOpenGraphMetadata(opengraph.New(
			title,
			"website",
			canonicalLink,
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
		)),
	)).ServeHTTP(res, req)
}

type FeaturesConsume struct {
	title templates.PageTitle
}

func HandleFeaturesConsume() http.HandlerFunc {
	return RenderExternalPage(&FeaturesConsume{
		title: templates.PageTitle{
			Summary:     "Consume",
			Description: "Features | RSS Reader, Newsletter Aggregator & Feed Organiser",
		},
	})
}

func (p *FeaturesConsume) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Consume | Foragd Features | RSS Reader, Newsletter Aggregator & Feed Organiser"
	description := "Discover Foragd's features focused around consumption: fetch content directly from the source, customise the UI and more."
	canonicalLink := config.GetBaseURL() + req.URL.String()
	templ.Handler(templates.CreatePage(templates.FeaturesPageConsume(),
		templates.WithPageTitle(p.title),
		templates.WithPageDescription(description),
		templates.WithCanonicalLink(canonicalLink),
		templates.WithOpenGraphMetadata(opengraph.New(
			title,
			"website",
			canonicalLink,
			config.GetBaseURL()+"/content/logo-vertical-light.webp",
			opengraph.WithDescription(description),
			opengraph.WithSiteName(config.AppName),
		)),
	)).ServeHTTP(res, req)
}
