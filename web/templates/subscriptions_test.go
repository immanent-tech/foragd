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

func TestLatestArticles_Empty(t *testing.T) {
	doc := render(t, t.Context(), templates.SubscriptionLatestArticles(nil))
	if doc.Find("p").Text() != "No entries." {
		t.Fatal("invalid output")
	}
}

func TestListMarkAllButton_Label(t *testing.T) {
	var buf bytes.Buffer
	_ = templates.ListMarkAllSubscriptionsButton(templates.MarkAction{Path: "/x", Label: "Mark All Read", Read: true}).
		Render(context.Background(), &buf)
	doc, _ := goquery.NewDocumentFromReader(&buf)
	if doc.Find("button[hx-post='/x']").Length() != 1 {
		t.Fatal("missing button")
	}
	if !strings.Contains(doc.Text(), "Mark All Read") {
		t.Fatal("missing label")
	}
}

func TestListMarkButton_Label(t *testing.T) {
	var buf bytes.Buffer
	_ = templates.ListMarkSubscriptionButton(templates.MarkAction{Path: "/x", Label: "Mark All Read", Read: true}).
		Render(context.Background(), &buf)
	doc, _ := goquery.NewDocumentFromReader(&buf)
	if doc.Find("button[hx-post='/x']").Length() != 1 {
		t.Fatal("missing button")
	}
	if !strings.Contains(doc.Text(), "Mark All Read") {
		t.Fatal("missing label")
	}
}

func newListSubscriptionsResponse(view models.View, subs ...*models.Subscription) *models.ListSubscriptionsResponse {
	r := &models.ListSubscriptionsResponse{}
	r.Filters.View = view
	r.Subscriptions = subs
	return r
}

func TestListSubscriptions(t *testing.T) {
	sub := func(id string, titles ...string) *models.Subscription {
		articles := make([]*models.Article, 0, len(titles))
		for title := range slices.Values(titles) {
			articles = append(articles, &models.Article{
				Item: models.Item{
					ItemID: title,
					Title:  title,
				},
			})
		}
		return &models.Subscription{
			SubscriptionID: id,
			Articles:       articles,
		}
	}

	tests := []struct {
		name         string
		resp         *models.ListSubscriptionsResponse
		setupCtx     func() context.Context
		wantCards    int
		wantEmpty    bool
		wantMarkPath string // "" when the bulk button shouldn't exist
		wantArticles int
	}{
		{
			name: "empty list shows no-content and no grid",
			resp: newListSubscriptionsResponse(models.ViewUnread),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards: 0,
			wantEmpty: true,
		},
		{
			name: "unread view offers mark-all-read",
			resp: newListSubscriptionsResponse(models.ViewUnread, sub("a", "One", "Two"), sub("b")),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards:    2,
			wantMarkPath: "/subscriptions/" + string(models.MarkRead),
			wantArticles: 2,
		},
		{
			name: "all view offers mark-all-unread",
			resp: newListSubscriptionsResponse(models.ViewAll, sub("a", "One")),
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			wantCards:    1,
			wantMarkPath: "/subscriptions/" + string(models.MarkUnread),
			wantArticles: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.ListSubscriptions(tc.resp))

			if got := doc.Find("#grid-objects .card").Length(); got != tc.wantCards {
				t.Errorf("cards: got %d, want %d", got, tc.wantCards)
			}

			hasGrid := doc.Find(".masonry-grid").Length() > 0
			if hasGrid == tc.wantEmpty {
				t.Errorf("grid present = %v, but wantEmpty = %v", hasGrid, tc.wantEmpty)
			}

			if tc.wantMarkPath == "" {
				if n := doc.Find("button[hx-post^='/subscriptions/mark']").Length(); n != 0 {
					t.Errorf("unexpected bulk button (%d)", n)
				}
			} else if doc.Find(`button[hx-post="`+tc.wantMarkPath+`"]`).Length() != 2 {
				t.Errorf("missing bulk button posting to %s", tc.wantMarkPath)
			}

			if got := doc.Find(".latest-items a[hx-get^='/articles/']").Length(); got != tc.wantArticles {
				t.Errorf("article links: got %d, want %d", got, tc.wantArticles)
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
