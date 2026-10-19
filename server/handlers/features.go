// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
)

type Features struct {
	*Page
}

func (m *Manager) HandleFeatures() http.HandlerFunc {
	features := &Features{
		Page: m.NewPage(
			"/features",
			WithSummary("Features"),
			WithDescription("RSS Reader, Newsletter Aggregator & Feed Organiser"),
			WithMetadataDescription(
				"Discover Foragd's features: subscribe to any RSS feed, YouTube channel, Google News search or email newsletter, organise with smart folders, and read distraction-free. No ads, no algorithms.",
			),
			WithTemplate(templates.Features()),
		),
	}
	return features.Handle()
}

type FeaturesCollect struct {
	*Page
}

func (m *Manager) HandleFeaturesCollect() http.HandlerFunc {
	collect := &FeaturesCollect{
		Page: m.NewPage(
			"/features/collect",
			WithSummary("Collect"),
			WithDescription("Features | RSS Reader, Newsletter Aggregator & Feed Organiser"),
			WithMetadataDescription(
				"Discover Foragd's features focused around collection: add any website, blog, YouTube channel, Google News search, or email newsletter easily.",
			),
			WithTemplate(templates.FeaturesPageCollect(m.AppConfig.GetBaseURL())),
		),
	}
	return collect.Handle()
}

type FeaturesCurate struct {
	*Page
}

func (m *Manager) HandleFeaturesCurate() http.HandlerFunc {
	curate := &FeaturesCollect{
		Page: m.NewPage(
			"/features/curate",
			WithSummary("Curate"),
			WithDescription("Features | RSS Reader, Newsletter Aggregator & Feed Organiser"),
			WithMetadataDescription(
				"Discover Foragd's features focused around curation: group subscriptions, save searches as subscriptions and filter articles easily.",
			),
			WithTemplate(templates.FeaturesPageCurate(m.AppConfig.GetBaseURL())),
		),
	}
	return curate.Handle()
}

type FeaturesConsume struct {
	*Page
}

func (m *Manager) HandleFeaturesConsume() http.HandlerFunc {
	consume := &FeaturesCollect{
		Page: m.NewPage(
			"/features/consume",
			WithSummary("Consume"),
			WithDescription("Features | RSS Reader, Newsletter Aggregator & Feed Organiser"),
			WithMetadataDescription(
				"Discover Foragd's features focused around consumption: fetch content directly from the source, customise the UI and more.",
			),
			WithTemplate(templates.FeaturesPageConsume(m.AppConfig.GetBaseURL())),
		),
	}
	return consume.Handle()
}
