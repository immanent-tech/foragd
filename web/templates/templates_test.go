/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

//go:generate go tool mockery
package templates_test

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"slices"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// render renders a component and returns a parsed DOM for assertions.
func render(t *testing.T, ctx context.Context, c templ.Component) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

// renderFragment renders a fragment and returns a parsed DOM for assertions.
func renderFragment(t *testing.T, ctx context.Context, c templ.Component, id any) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := templ.RenderFragments(ctx, &buf, c, id); err != nil {
		t.Fatalf("render fragment: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func genArticles(t *testing.T, titles ...string) models.Articles {
	t.Helper()
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

func genSubscriptions(t *testing.T, titles ...string) models.Subscriptions {
	t.Helper()
	subscriptions := make(models.Subscriptions, 0, len(titles))
	for title := range slices.Values(titles) {
		subscriptions = append(subscriptions, &models.Subscription{
			SubscriptionID: title,
		})
	}
	return subscriptions
}

func defaultAppCfg() *MoqAppConfig {
	return &MoqAppConfig{
		GetAppIDFunc:      func() string { return "app.test" },
		GetAppNameFunc:    func() string { return "Test App" },
		GetAppVersionFunc: func() string { return "v0.0.0" },
		GetBaseURLFunc: func() *url.URL {
			baseURL, _ := url.Parse("https://test.com")
			return baseURL
		},
		IsProductionFunc: func() bool { return true },
	}
}

type TestTempl struct{}

func (t *TestTempl) Render(ctx context.Context, w io.Writer) error {
	w.Write([]byte(`<div id="test-body">Test</div>`))
	return nil
}

func TestCreatePage(t *testing.T) {
	tests := []struct {
		name     string
		appCfg   templates.AppConfig
		session  templates.SessionManager
		page     templates.Page
		setupCtx func() context.Context
		tests    func(t *testing.T, doc *goquery.Document)
	}{
		{
			name: "with bare minimum page",
			setupCtx: func() context.Context {
				return models.UserToCtx(t.Context(), &models.User{})
			},
			session: &MoqSessionManager{
				GetFunc: func(ctx context.Context, key string) any {
					return ""
				},
			},
			appCfg: defaultAppCfg(),
			page: &MoqPage{
				GetDescriptionFunc: func() string {
					return "A Test Page"
				},
				GetTitleFunc: func() string {
					return "Test App"
				},
				CanonicalLinkFunc: func() string {
					return "https://test.com/page"
				},
				GetTemplateFunc: func() templ.Component {
					return &TestTempl{}
				},
				ToMetaTagsFunc: func() templ.Component {
					return &TestTempl{}
				},
				ToJsonLdFunc: func() templ.Component {
					return &TestTempl{}
				},
			},
			tests: func(t *testing.T, doc *goquery.Document) {
				if got := doc.Find("title").Text(); got != "Test App" {
					t.Errorf("get title: got %s, want %s", got, "Test App")
				}
				if got := doc.Find("head meta[name='description'][content*='A Test Page']").Length(); got != 1 {
					t.Errorf("get meta description: got %d, want %d", got, 1)
				}
				if got := doc.Find("head link[rel='canonical'][href='https://test.com/page']").Length(); got != 1 {
					t.Errorf("get canonical link: got %d, want %d", got, 1)
				}
				if got := doc.Find("head link[rel='alternate'][href='https://test.com/feed']").Length(); got != 1 {
					t.Errorf("get feed link: got %d, want %d", got, 1)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupCtx()
			doc := render(t, ctx, templates.CreatePage(tc.appCfg, tc.session, tc.page))
			tc.tests(t, doc)
		})
	}
}
