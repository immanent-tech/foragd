/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/middlewares"
)

func BenchmarkCanonicalizeListFilters(b *testing.B) {
	session := &MoqSessionManager{}
	mw := middlewares.CanonicalizeListFilters(session)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/articles?sort=NewestFirst&view=Unread&count=10", nil)
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req)
	}
}

func BenchmarkCanonicalizeListFilters_HistoryRestore(b *testing.B) {
	session := &MoqSessionManager{
		GetFunc: func(ctx context.Context, key string) any {
			switch key {
			case "listFilters":
				return models.NewListFilters()
			case "listCount":
				return 9
			}
			return nil
		},
		PutFunc: func(ctx context.Context, key string, value any) {},
	}
	mw := middlewares.CanonicalizeListFilters(session)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/subscriptions?sort=LeastUnread", nil)
		// Simulate history restore request
		req.Header.Set(models.ActionHeader, "mark-subscription")
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req)
	}
}

func BenchmarkCanonicalizeListFilters_POST(b *testing.B) {
	session := &MoqSessionManager{
		GetFunc: func(ctx context.Context, key string) any {
			switch key {
			case "listFilters":
				filters := models.NewListFilters()
				filters.From = new(10)
			case "listCount":
				return 9
			}
			return nil
		},
		PutFunc: func(ctx context.Context, key string, value any) {},
	}
	mw := middlewares.CanonicalizeListFilters(session)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	form := url.Values{}
	form.Set("sort", "newest_first")
	form.Set("count", "9")
	form.Add("view", "unread")
	form.Add("from", "20")

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req)
	}
}

func BenchmarkParseListFilters(b *testing.B) {
	query := url.Values{}
	query.Set("sort", "newest_first")
	query.Set("view", "unread")
	query.Set("count", "10")
	query.Set("category", "Tech")
	query.Set("subscriptions", "sub1,sub2,sub3")

	b.ReportAllocs()
	for b.Loop() {
		_ = models.ParseListFilters(query)
	}
}

func BenchmarkParseListFilters_Empty(b *testing.B) {
	query := url.Values{}

	b.ReportAllocs()
	for b.Loop() {
		_ = models.ParseListFilters(query)
	}
}

// Benchmark for the hot path: ParseListFilters + Sanitise
func BenchmarkParseListFilters_Sanitise(b *testing.B) {
	query := url.Values{}
	query.Set("sort", "newest_first")
	query.Set("view", "unread")
	query.Set("count", "10")
	query.Set("category", "Tech")
	query.Set("subscriptions", "sub1,sub2,sub3")

	b.ReportAllocs()
	for b.Loop() {
		filters := models.ParseListFilters(query)
		_ = filters.Sanitise()
	}
}
