/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"context"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestUserHome(t *testing.T) {
	tests := []struct {
		name     string
		data     templates.HomeData
		setupCtx func() context.Context
		tests    func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "no data",
			data: templates.HomeData{},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if doc.Find("#no-content").Length() != 1 {
					t.Errorf("not content hero not shown")
				}
			},
		},
		{
			name: "with data",
			data: templates.HomeData{
				LatestArticles: genArticles(t, "One", "Two"),
				Subscriptions:  genSubscriptions(t, "Foo", "Bar"),
				TopCategories: models.CategoryCounts{
					models.CategoryCount{Category: "Stuff"},
				},
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if length := doc.Find("#latest-articles > .masonry-grid > .card").Length(); length != 2 {
					t.Errorf("failed article list: expected %d, got %d", 2, length)
				}
				if length := doc.Find("#latest-subscriptions > li").Length(); length != 2 {
					t.Errorf("failed subscription list: expected %d, got %d", 2, length)
				}
				if length := doc.Find("#top-categories > .badge").Length(); length != 1 {
					t.Errorf("failed subscription list: expected %d, got %d", 1, length)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.UserHome(&tc.data))
			tc.tests(t, doc)
		})
	}
}
