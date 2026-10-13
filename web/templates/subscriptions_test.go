/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"

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

func TestEditFeedSubscription(t *testing.T) {
	tests := []struct {
		name        string
		request     *models.FeedSubscriptionRequest
		breadcrumbs templates.Breadcrumbs
		setupCtx    func() context.Context
		tests       func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "just nickname",
			request: &models.FeedSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{
					Nickname: new("Test"),
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Editing Test")
				assert.Equal(t, doc.Find("input[value^='Test']").Length(), 1)
			},
		},
		{
			name: "with article filtering",
			request: &models.FeedSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{
					Nickname: new("Test"),
				},
				ArticleFilters: &models.ArticleFilters{
					Text: new(`-foo "bar baz"`),
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Editing Test")
				assert.Equal(t, doc.Find("input[value^='Test']").Length(), 1)
				assert.True(t, doc.Find("#article-text-filters").Text() == `-foo "bar baz"`)
			},
		},
		{
			name: "with categories",
			request: &models.FeedSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{
					Nickname:            new("Test"),
					Categories:          []models.Category{"Pop Culture", "Events", "Custom"},
					SuggestedCategories: []models.Category{"Suggested1"},
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Editing Test")
				assert.Equal(t, doc.Find("input[value^='Test']").Length(), 1)
				assert.Equal(t, doc.Find(`input[type="checkbox"][value="Pop Culture"][checked]`).Length(), 1)
				assert.Equal(t, doc.Find(`input[type="checkbox"][value="Events"][checked]`).Length(), 1)
				assert.Equal(t, doc.Find(`input[type="checkbox"][value="Custom"][checked]`).Length(), 1)
				assert.Equal(t, doc.Find(`input[type="checkbox"][value="Suggested1"]`).Length(), 1)
			},
		},
		{
			name: "with back navigation",
			request: &models.FeedSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{
					Nickname: new("Test"),
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) {
					prevURL, _ := url.Parse("/custom/back")
					return prevURL, true
				},
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, doc.Find("button[hx-get^='/custom/back']").Length(), 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.EditFeedSubscription(tc.request, tc.breadcrumbs))
			tc.tests(t, doc)
		})
	}
}

func TestAddSearchSubscription(t *testing.T) {
	tests := []struct {
		name        string
		request     *models.SearchSubscriptionRequest
		breadcrumbs templates.Breadcrumbs
		setupCtx    func() context.Context
		tests       func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "no search",
			request: &models.SearchSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Add a Search Subscription")
				assert.Equal(t, doc.Find("form[hx-post^='/subscriptions/add/search']").Length(), 1)
			},
		},
		{
			name: "with search",
			request: &models.SearchSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{},
				Search: models.SearchRequest{
					Text: `-foo "bar baz"`,
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Add a Search Subscription")
				assert.True(t, strings.TrimSpace(doc.Find("#search-text").Text()) == `-foo "bar baz"`)
				assert.Equal(t, doc.Find("input[name='count'][value='9']").Length(), 1)
				assert.Equal(t, doc.Find("form[hx-post^='/subscriptions/add/search']").Length(), 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.AddSearchSubscription(tc.request, tc.breadcrumbs))
			tc.tests(t, doc)
		})
	}
}

func TestEditSearchSubscription(t *testing.T) {
	tests := []struct {
		name        string
		request     *models.SearchSubscriptionRequest
		breadcrumbs templates.Breadcrumbs
		setupCtx    func() context.Context
		tests       func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "with search",
			request: &models.SearchSubscriptionRequest{
				Customisation: &models.SubscriptionCustomisation{
					Nickname: new("Test"),
				},
				Search: models.SearchRequest{
					Text:           `-foo "bar baz"`,
					SubscriptionID: new("sub_test"),
				},
			},
			breadcrumbs: &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) { return nil, false },
			},
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				assert.True(t, strings.TrimSpace(doc.Find("#page-heading").Text()) == "Editing Test")
				assert.True(t, strings.TrimSpace(doc.Find("#search-text").Text()) == `-foo "bar baz"`)
				assert.Equal(t, doc.Find("input[name='count'][value='9']").Length(), 1)
				assert.Equal(t, doc.Find("input[name='subscription_id'][value='sub_test']").Length(), 1)
				assert.Equal(t, doc.Find("input[name='subscription_type'][value='search']").Length(), 1)
				assert.Equal(t, doc.Find("form[hx-post^='/subscriptions/sub_test/edit']").Length(), 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.EditSearchSubscription(tc.request, tc.breadcrumbs))
			tc.tests(t, doc)
		})
	}
}
