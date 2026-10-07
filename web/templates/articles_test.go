/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestListMarkAllArticlesButton_Label(t *testing.T) {
	var buf bytes.Buffer
	_ = templates.ListMarkAllArticlesButton(templates.MarkAction{Path: "/x", Label: "Mark All Read", Read: true}).
		Render(context.Background(), &buf)
	doc, _ := goquery.NewDocumentFromReader(&buf)
	if doc.Find("button[hx-post='/x']").Length() != 1 {
		t.Fatal("missing button")
	}
	if !strings.Contains(doc.Text(), "Mark All Read") {
		t.Fatal("missing label")
	}
}

// newResponse builds a fixture. Adjust to your real model constructors.
func newListArticlesResponse(
	view models.View,
	subscription *models.Subscription,
	articles ...*models.Article,
) *models.ListArticlesResponse {
	r := &models.ListArticlesResponse{}
	r.Filters.View = view
	r.Articles = articles
	return r
}

func TestListArticles(t *testing.T) {
	genArticles := func(id string, titles ...string) models.Articles {
		articles := make([]*models.Article, 0, len(titles))
		for title := range slices.Values(titles) {
			articles = append(articles, &models.Article{
				Item: models.Item{
					ItemID: title,
					Title:  title,
				},
				State: &models.ArticleState{},
			})
		}
		return articles
	}

	tests := []struct {
		name         string
		resp         *models.ListArticlesResponse
		setupCtx     func() context.Context
		wantCards    int
		wantEmpty    bool
		wantMarkPath string // "" when the bulk button shouldn't exist
		wantArticles int
	}{
		{
			name: "empty list shows no-content and no grid",
			resp: newListArticlesResponse(models.ViewUnread, nil),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards: 0,
			wantEmpty: true,
		},
		{
			name: "unread view offers mark-all-read",
			resp: newListArticlesResponse(models.ViewUnread, nil, genArticles("a", "One", "Two")...),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards:    2,
			wantMarkPath: "/articles/" + string(models.MarkRead),
			wantArticles: 2,
		},
		{
			name: "all view offers mark-all-unread",
			resp: newListArticlesResponse(models.ViewAll, nil, genArticles("a", "One")...),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards:    1,
			wantMarkPath: "/articles/" + string(models.MarkUnread),
			wantArticles: 1,
		},
		{
			name: "unread view offers mark-all-read with subscription",
			resp: newListArticlesResponse(
				models.ViewUnread,
				&models.Subscription{SubscriptionID: "sub"},
				genArticles("a", "One", "Two")...),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards:    2,
			wantMarkPath: "/articles/" + string(models.MarkRead),
			wantArticles: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.ListArticles(tc.resp))

			if got := doc.Find("#grid-objects .card").Length(); got != tc.wantCards {
				t.Errorf("cards: got %d, want %d", got, tc.wantCards)
			}

			hasGrid := doc.Find(".masonry-grid").Length() > 0
			if hasGrid == tc.wantEmpty {
				t.Errorf("grid present = %v, but wantEmpty = %v", hasGrid, tc.wantEmpty)
			}

			if tc.resp.Subscription == nil {
				if n := doc.Find("button[hx-post^='/articles/mark']").Length(); n != 0 {
					t.Errorf("unexpected bulk button (%d)", n)
				}
			} else if doc.Find(`button[hx-post="/subscriptions`+tc.resp.Subscription.GetID()+`"]`).Length() != 1 {
				t.Errorf("missing bulk button posting to %s", tc.wantMarkPath)
			}

			// The poller must carry the current filters, or polling silently drifts.
			raw, ok := doc.Find("#updates").Attr("hx-vals")
			if !ok {
				t.Fatal("updates poller missing hx-vals")
			}
			var got, want map[string]any
			wantJSON, _ := json.Marshal(tc.resp.Filters)
			_ = json.Unmarshal([]byte(raw), &got)
			_ = json.Unmarshal(wantJSON, &want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("hx-vals mismatch:\n got %v\nwant %v", got, want)
			}
		})
	}
}
