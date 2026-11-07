/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestSearchSuggestions(t *testing.T) {
	tests := []struct {
		name    string
		results models.SearchResults
		tests   func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "subscription suggestion",
			results: models.SearchResults{
				Search: models.SearchRequest{
					Text: "test",
				},
				Subscriptions: models.Subscriptions{
					{
						SubscriptionID: "sub_test",
					},
				},
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if got := doc.Find("#subscription-suggestions li").Length(); got != 2 {
					t.Errorf("subscription suggestions: got %d, want %d", got, 2)
				}
			},
		},
		{
			name: "article suggestion",
			results: models.SearchResults{
				Search: models.SearchRequest{
					Text: "test",
				},
				Articles: models.Articles{
					{Item: models.Item{
						Title:  "test article",
						ItemID: "item_test",
					}},
				},
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if got := doc.Find("#article-suggestions li").Length(); got != 2 {
					t.Errorf("article suggestions: got %d, want %d", got, 2)
				}
			},
		},
		{
			name: "no suggestions",
			results: models.SearchResults{
				Search: models.SearchRequest{
					Text: "test",
				},
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if got := doc.Find("ul > li").Length(); got != 1 {
					t.Errorf("article suggestions: got %d, want %d", got, 1)
				}
			},
		},
		{
			name: "action suggestions",
			results: models.SearchResults{
				Search: models.SearchRequest{
					Text: "add",
				},
				Articles: models.Articles{
					{Item: models.Item{
						Title:  "test article",
						ItemID: "item_test",
					}},
				},
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if got := doc.Find("#article-suggestions li").Length(); got != 2 {
					t.Errorf("article suggestions: got %d, want %d", got, 2)
				}
				if got := doc.Find("#action-suggestions li").Length(); got != 4 {
					t.Errorf("article suggestions: got %d, want %d", got, 4)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := render(t, t.Context(), templates.SearchSuggestions(&tc.results))
			// Check for full search action.
			value, found := doc.Find("#suggestion-text").Attr("value")
			if !found {
				t.Error("full search suggestion not found")
			}
			if value != tc.results.Search.Text {
				t.Error("full search suggestion text value missing")
			}
			// Specific tests.
			tc.tests(t, doc)
		})
	}
}
