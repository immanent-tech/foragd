/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
)

func TestManager_CanonicalizeListFilters(t *testing.T) {
	tests := []struct {
		name       string
		method     string              // method for request.
		path       string              // path for request.
		headers    map[string]string   // additional headers to set in request.
		have       *models.ListFilters // filters passed in.
		want       *models.ListFilters // expected filters.
		nextCalled bool                // whether the next handler should be called.
		sessionMgr handlers.SessionManager
		ctx        func(ctx context.Context) context.Context
	}{
		{
			name:       "not list page",
			method:     http.MethodGet,
			path:       "/home",
			want:       models.NewListFilters(),
			nextCalled: true,
		},
		{
			name:       "default filters",
			method:     http.MethodGet,
			path:       "/subscriptions",
			have:       models.NewListFilters(),
			want:       models.NewListFilters(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
		{
			name:       "empty filters",
			method:     http.MethodGet,
			path:       "/subscriptions",
			want:       models.NewListFilters(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: false,
		},
		{
			name:   "invalid count",
			method: http.MethodGet,
			path:   "/subscriptions",
			have: &models.ListFilters{
				Count: 1000,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
			},
			want: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
			},
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: false,
		},
		{
			name:   "invalid upto",
			method: http.MethodPost,
			path:   "/subscriptions",
			have: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewUnread,
				UpTo:  new(5000),
			},
			want: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewUnread,
				UpTo:  new(1000),
			},
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
		{
			name:   "back button navigation (subscriptions)",
			method: http.MethodGet,
			path:   "/subscriptions",
			headers: map[string]string{
				htmx.HeaderHistoryRestoreRequest: "true",
			},
			have: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
			},
			want: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
				UpTo:  new(27),
			},
			sessionMgr: newMoqSessionMap(map[string]any{
				"listFilters/subscriptions": models.ListFilters{
					Count: 9,
					Sort:  models.SortNewestFirst,
					View:  models.ViewAll,
					From:  new(18),
				},
			}),
			nextCalled: true,
		},
		{
			name:   "back button navigation (articles)",
			method: http.MethodGet,
			path:   "/articles",
			headers: map[string]string{
				htmx.HeaderHistoryRestoreRequest: "true",
			},
			have: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
			},
			want: &models.ListFilters{
				Count: 9,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
				UpTo:  new(18),
			},
			sessionMgr: newMoqSessionMap(map[string]any{
				"listFilters/articles": models.ListFilters{
					Count: 9,
					Sort:  models.SortNewestFirst,
					View:  models.ViewAll,
					From:  new(9),
				},
			}),
			nextCalled: true,
		},
		{
			name:       "empty post",
			method:     http.MethodPost,
			path:       "/subscriptions",
			want:       models.NewListFilters(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up context.
			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx(ctx)
			}

			// Set up middleware.
			var gotReq *http.Request
			h := middlewares.CanonicalizeListFilters(tt.sessionMgr)(
				http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					gotReq = req
				}),
			)
			// Set up request.
			var req *http.Request
			switch tt.method {
			case http.MethodGet:
				req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, nil)
				if tt.have != nil {
					req.URL.RawQuery = tt.have.Encode()
				}
			case http.MethodPost:
				if tt.have != nil {
					req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, strings.NewReader(tt.have.Encode()))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, nil)
				}
			}
			if tt.headers != nil {
				for k, v := range tt.headers {
					req.Header.Set(k, v)
				}
			}
			rec := httptest.NewRecorder()
			// Call middleware.
			h.ServeHTTP(rec, req)

			if tt.nextCalled {
				if gotReq == nil {
					t.Fatal("next handler should be called")
				}
				// Check context values properly set.
				if got := handlers.ListFiltersFromCtx(gotReq.Context()); got.Encode() != tt.want.Encode() {
					t.Errorf("Filters context = %q, want %q", got.Encode(), tt.want.Encode())
				}
				if middlewares.IsListPage(tt.path) {
					// For list pages, check session values properly set.
					if got := handlers.ListFiltersFromSession(
						req.Context(),
						tt.sessionMgr,
						tt.path,
					); got.Encode() != tt.want.Encode() {
						t.Errorf("Filters session = %q, want %q", got.Encode(), tt.want.Encode())
					}
					// For GET request, check HTMX PushURL header set.
					if tt.method == http.MethodGet {
						// For
						if pushURL := rec.Header().
							Get(htmx.HeaderPushURL); pushURL != tt.path+"?"+handlers.ListFiltersFromCtx(gotReq.Context()).
							Encode() {
							t.Errorf(
								"Push URL = %q, want %q",
								pushURL,
								handlers.ListFiltersFromCtx(gotReq.Context()).Encode(),
							)
						}
					}
				}
			} else {
				// For other pages, check redirect.
				if rec.Code != http.StatusFound {
					t.Fatal("next handler not called and not redirected")
				}
			}
		})
	}
}
