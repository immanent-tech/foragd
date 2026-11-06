// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/stretchr/testify/assert"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
)

func TestManager_CanonicalizeSearchParams(t *testing.T) {
	tests := []struct {
		name       string
		method     string            // method for request.
		path       string            // path for request.
		headers    map[string]string // additional headers to set in request.
		have       *models.SearchRequest
		want       *models.SearchRequest
		nextCalled bool // whether the next handler should be called.
		sessionMgr handlers.SessionManager
		ctx        func(ctx context.Context) context.Context
	}{
		{
			name:       "default search (htmx)",
			method:     http.MethodGet,
			path:       "/search",
			have:       models.NewSearchRequest(),
			want:       models.NewSearchRequest(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
			headers: map[string]string{
				htmx.HeaderRequest: "true",
			},
		},
		{
			name:       "default search",
			method:     http.MethodGet,
			path:       "/search",
			have:       models.NewSearchRequest(),
			want:       models.NewSearchRequest(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
		{
			name:       "no search params (get)",
			method:     http.MethodGet,
			path:       "/search",
			want:       models.NewSearchRequest(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: false,
		},
		{
			name:       "no search params (post)",
			method:     http.MethodPost,
			path:       "/search",
			want:       models.NewSearchRequest(),
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
		{
			name:   "back button navigation",
			method: http.MethodGet,
			path:   "/search",
			headers: map[string]string{
				htmx.HeaderHistoryRestoreRequest: "true",
			},
			want: &models.SearchRequest{
				Text:  "test",
				From:  0,
				Sort:  models.SortNewestFirst,
				View:  models.ViewAll,
				Count: new(18),
			},
			sessionMgr: newMoqSessionMap(map[string]any{
				"searchParams": models.SearchRequest{
					Sort:        models.SortNewestFirst,
					SearchAfter: new("text"),
					View:        models.ViewAll,
					Text:        "test",
					From:        18,
				},
			}),
			nextCalled: true,
		},
		{
			name:   "pagination",
			method: http.MethodPost,
			path:   "/search",
			have: &models.SearchRequest{
				Text:          "test",
				From:          9,
				SearchAfter:   new("test"),
				Subscriptions: []models.SubscriptionID{"sub_test"},
				Sort:          models.SortNewestFirst,
				View:          models.ViewUnread,
			},
			want: &models.SearchRequest{
				Text:            "test",
				From:            9,
				SearchAfter:     new("test"),
				Subscriptions:   []models.SubscriptionID{"sub_test"},
				Sort:            models.SortNewestFirst,
				View:            models.ViewUnread,
				PublishedWithin: models.SearchRequestPublishedWithinLastWeek,
				Timezone:        "UTC",
			},
			sessionMgr: newMoqSessionMap(nil),
			nextCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			// Set up context.
			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx(ctx)
			}

			// Set up middleware.
			var gotReq *http.Request
			h := middlewares.CanonicalizeSearchParams(tt.sessionMgr)(
				http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
					gotReq = req
				}),
			)
			// Set up request.
			var (
				req       *http.Request
				urlHeader string
			)
			switch tt.method {
			case http.MethodGet:
				req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, nil)
				if tt.have != nil {
					req.URL.RawQuery = tt.have.Encode()
				}
				urlHeader = htmx.HeaderPushURL
			case http.MethodPost:
				if tt.have != nil {
					req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, strings.NewReader(tt.have.Encode()))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					req = httptest.NewRequestWithContext(ctx, tt.method, tt.path, nil)
				}
				urlHeader = htmx.HeaderReplaceUrl
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
				gotCtx := handlers.SearchParamsFromCtx(gotReq.Context())
				if gotCtx.Encode() != tt.want.Encode() {
					t.Errorf("Search context = %q, want %q", gotCtx.Encode(), tt.want.Encode())
				}
				assert.Equal(tt.want.GetFrom(), gotCtx.GetFrom(), "From")
				assert.Equal(tt.want.GetCount(), gotCtx.GetCount(), "Count")
				assert.Equal(tt.want.SearchAfter, gotCtx.SearchAfter, "SearchAfter")
				// Check session values properly set.
				gotSession := handlers.SearchParamsFromSession(
					req.Context(),
					tt.sessionMgr,
				)
				if gotSession.Encode() != tt.want.Encode() {
					t.Errorf("Search session = %q, want %q", gotSession.Encode(), tt.want.Encode())
				}
				assert.Equal(tt.want.GetFrom(), gotSession.GetFrom(), "From")
				assert.Equal(tt.want.GetCount(), gotSession.GetCount(), "Count")
				assert.Equal(tt.want.SearchAfter, gotSession.SearchAfter, "SearchAfter")
				// Check HTMX PushURL header set.
				if newURL := rec.Header().
					Get(urlHeader); newURL != tt.path+"?"+handlers.SearchParamsFromCtx(gotReq.Context()).
					Encode() {
					t.Errorf(
						"Push URL = %q, want %q",
						newURL,
						handlers.SearchParamsFromCtx(gotReq.Context()).Encode(),
					)
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
