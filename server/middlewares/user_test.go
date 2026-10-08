/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/immanent-tech/foragd/server/middlewares"
)

// TestSkipUpdatesRoute tests the public SkipUpdatesRoute function.
func TestSkipUpdatesRoute(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectWrites bool
	}{
		{"updates path", "/updates", true},
		{"updates nested", "/updates/feed", true},
		{"updates trailing slash", "/updates/", true},
		{"non-updates path", "/subscriptions", false},
		{"other route", "/login", false},
		{"subscribed path", "/subscribe", false},
		{"api path", "/api/data", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			res := httptest.NewRecorder()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := middlewares.SkipUpdatesRoute(next)
			result := middleware(res, req)

			if tt.expectWrites {
				assert.Equal(t, http.StatusOK, res.Code, "expected write for %s", tt.name)
				assert.True(t, result, "expected true for %s", tt.name)
			} else {
				assert.NotEqual(t, http.StatusOK, res.Code, "expected no write for %s", tt.name)
				assert.False(t, result, "expected false for %s", tt.name)
			}
		})
	}
}

// TestExtractUserFromSession tests the public ExtractUserFromSession function.
func TestExtractUserFromSession(t *testing.T) {
	t.Run("skips updates route", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/updates/test", nil)
		res := httptest.NewRecorder()

		middleware := middlewares.ExtractUserFromSession(nil, nil, nil)
		handler := middleware(next)
		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusOK, res.Code, "should skip updates route")
	})
}

// TestRequireValidUser tests the public RequireValidUser function.
func TestRequireValidUser(t *testing.T) {
	handler := middlewares.RequireValidUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	assert.NotNil(t, handler)
}
