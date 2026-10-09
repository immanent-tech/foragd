/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/middlewares"
)

type fakeAuth struct {
	*MoqAuthenticator

	ensureErr error
	returnTo  []string
	cleared   int
}

func NewFakeAuth() *fakeAuth {
	f := &fakeAuth{}
	f.MoqAuthenticator = &MoqAuthenticator{
		EnsureFreshFunc: func(ctx context.Context) error {
			return f.ensureErr
		},
		PutReturnToFunc: func(ctx context.Context, path string) {
			f.returnTo = append(f.returnTo, path)
		},
		ClearAuthFunc: func(ctx context.Context) {
			f.cleared++
		},
	}
	return f
}

type fakeUsers struct {
	*MoqUserService

	user  *models.User
	err   error
	calls int
	gotID string
}

func NewFakeUsers() *fakeUsers {
	f := &fakeUsers{}
	f.MoqUserService = &MoqUserService{
		GetUserByExternalIDFunc: func(ctx context.Context, externalID string) (*models.User, error) {
			f.calls++
			f.gotID = externalID
			return f.user, f.err
		},
	}
	return f
}

type fakeSession struct {
	*MoqSessionManager

	profile any
}

func NewFakeSession() *fakeSession {
	f := &fakeSession{}
	f.MoqSessionManager = &MoqSessionManager{
		GetFunc: func(ctx context.Context, key string) any {
			if key == "profile" {
				return f.profile
			}
			return nil
		},
	}
	return f
}

// testProfile builds a profile the way production does: by decoding ID token claims. If these assertions fail, adjust
// the JSON to match the claim names of models.UserProfileResponse.
func testProfile(t *testing.T, blocked bool) models.UserProfileResponse {
	t.Helper()
	var p models.UserProfileResponse
	raw := fmt.Sprintf(`{"sub":"auth0|123","blocked":%t}`, blocked)
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if p.GetID() == "" {
		t.Fatal("profile ID is empty: adjust testProfile's JSON to match models.UserProfileResponse")
	}
	if blocked && (p.Blocked == nil || !*p.Blocked) {
		t.Fatal("profile is not blocked: adjust testProfile's JSON to match models.UserProfileResponse")
	}
	return p
}

type result struct {
	rec        *httptest.ResponseRecorder
	nextCalled bool
	nextUser   *models.User
}

// run sends a GET /home?tab=1 request through ExtractUserFromSession.
func run(
	t *testing.T,
	auth *fakeAuth,
	session *fakeSession,
	users *fakeUsers,
	isHTMX bool,
) result {
	t.Helper()
	var res result
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.nextCalled = true
		res.nextUser = models.UserFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/home?tab=1", nil)
	if isHTMX {
		req.Header.Set("HX-Request", "true")
	}
	res.rec = httptest.NewRecorder()
	middlewares.ExtractUserFromSession(users, auth, session)(next).ServeHTTP(res.rec, req)
	return res
}

func bothTransports() []struct {
	name   string
	isHTMX bool
} {
	return []struct {
		name   string
		isHTMX bool
	}{{"browser", false}, {"htmx", true}}
}

// assertLoginRedirect checks the response sends the client to /login in the way suited to the transport.
func assertLoginRedirect(t *testing.T, res result, isHTMX bool) {
	t.Helper()
	assertRedirect(t, res, isHTMX, "/login", http.StatusFound, http.StatusUnauthorized)
}

func assertRedirect(t *testing.T, res result, isHTMX bool, target string, browserStatus, htmxStatus int) {
	t.Helper()
	if res.nextCalled {
		t.Error("next handler must not run after a redirect has been written")
	}
	if isHTMX {
		if res.rec.Code != htmxStatus {
			t.Errorf("status = %d, want %d", res.rec.Code, htmxStatus)
		}
		if got := res.rec.Header().Get(htmx.HeaderRedirect); got != target {
			t.Errorf("%s = %q, want %q", htmx.HeaderRedirect, got, target)
		}
		return
	}
	if res.rec.Code != browserStatus {
		t.Errorf("status = %d, want %d", res.rec.Code, browserStatus)
	}
	if got := res.rec.Header().Get("Location"); got != target {
		t.Errorf("Location = %q, want %q", got, target)
	}
}

func TestExtractUser_SessionExpiredRedirectsToLogin(t *testing.T) {
	for _, tr := range bothTransports() {
		t.Run(tr.name, func(t *testing.T) {
			auth := NewFakeAuth()
			auth.ensureErr = fmt.Errorf("refresh: %w", auth0.ErrSessionExpired)

			users := NewFakeUsers()

			res := run(t, auth, NewFakeSession(), users, tr.isHTMX)

			// Regression: the handler used to keep going (next.ServeHTTP) after writing the redirect.
			assertLoginRedirect(t, res, tr.isHTMX)
			if len(auth.returnTo) != 1 || auth.returnTo[0] != "/home?tab=1" {
				t.Errorf("return_to = %v, want [/home?tab=1]", auth.returnTo)
			}
			if users.calls != 0 {
				t.Error("user lookup must not happen for an unauthenticated request")
			}
		})
	}
}

func TestExtractUser_TransientRefreshErrorDoesNotLogOut(t *testing.T) {
	for _, tr := range bothTransports() {
		t.Run(tr.name, func(t *testing.T) {
			auth := NewFakeAuth()
			auth.ensureErr = errors.New("dial tcp: i/o timeout")
			users := NewFakeUsers()

			res := run(t, auth, NewFakeSession(), users, tr.isHTMX)

			if res.rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", res.rec.Code)
			}
			if res.rec.Header().Get("Retry-After") == "" {
				t.Error("Retry-After header missing")
			}
			if auth.cleared != 0 {
				t.Error("a transient Auth0 failure must not clear the user's session")
			}
			if len(auth.returnTo) != 0 {
				t.Error("return_to must not be set: the user is not being sent to login")
			}
			if res.nextCalled || users.calls != 0 {
				t.Error("request must stop at the middleware")
			}
		})
	}
}

func TestExtractUser_InvalidSessionProfileForcesRelogin(t *testing.T) {
	tests := []struct {
		name    string
		profile any
	}{
		{"missing", nil},
		{"unexpected type", "not-a-profile"},
		{"empty profile", models.UserProfileResponse{}},
		{"nil pointer", (*models.UserProfileResponse)(nil)},
	}
	for _, tc := range tests {
		for _, tr := range bothTransports() {
			t.Run(tc.name+"/"+tr.name, func(t *testing.T) {
				auth := NewFakeAuth()
				users := NewFakeUsers()
				session := NewFakeSession()
				session.profile = tc.profile

				res := run(t, auth, session, users, tr.isHTMX)

				assertLoginRedirect(t, res, tr.isHTMX)
				if auth.cleared != 1 {
					t.Errorf("ClearAuth calls = %d, want 1", auth.cleared)
				}
				if users.calls != 0 {
					t.Error("user lookup must not happen without a profile")
				}
			})
		}
	}
}

func TestExtractUser_BlockedUserIsRedirected(t *testing.T) {
	for _, tr := range bothTransports() {
		t.Run(tr.name, func(t *testing.T) {
			users := NewFakeUsers()
			session := NewFakeSession()
			session.profile = testProfile(t, true)
			res := run(t, NewFakeAuth(), session, users, tr.isHTMX)

			assertRedirect(t, res, tr.isHTMX, "/account-issue", http.StatusTemporaryRedirect, http.StatusOK)
			if users.calls != 0 {
				t.Error("blocked users must be stopped before the user lookup")
			}
		})
	}
}

func TestExtractUser_UserLookupFailureNeverReturnsBlankPage(t *testing.T) {
	for _, tr := range bothTransports() {
		t.Run(tr.name, func(t *testing.T) {
			auth := NewFakeAuth()
			users := NewFakeUsers()
			users.err = errors.New("database unavailable")
			session := NewFakeSession()
			session.profile = testProfile(t, false)

			res := run(t, auth, session, users, tr.isHTMX)

			// Regression: this used to return nothing at all, i.e. an empty 200 response.
			if res.rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", res.rec.Code)
			}
			if res.rec.Body.Len() == 0 {
				t.Error("response body is empty")
			}
			if res.nextCalled {
				t.Error("next handler must not run")
			}
			if auth.cleared != 0 {
				t.Error("a backend outage must not log the user out")
			}
		})
	}
}

func TestExtractUser_PassesUserToNextHandler(t *testing.T) {
	profile := testProfile(t, false)
	tests := []struct {
		name    string
		profile any
	}{
		{"profile stored by value", profile},
		{"profile stored by pointer", &profile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := &models.User{UserID: "user_1"}
			users := NewFakeUsers()
			users.user = want
			session := NewFakeSession()
			session.profile = tc.profile

			res := run(t, NewFakeAuth(), session, users, false)

			if !res.nextCalled {
				t.Fatalf("next handler was not called (status %d)", res.rec.Code)
			}
			if res.rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", res.rec.Code)
			}
			if res.nextUser != want {
				t.Error("user was not placed in the request context")
			}
			if users.gotID != profile.GetID() {
				t.Errorf("looked up %q, want %q", users.gotID, profile.GetID())
			}
		})
	}
}

func TestRequireValidUser_Rejections(t *testing.T) {
	tests := []struct {
		name         string
		user         *models.User
		wantStatus   int
		wantLocation string
	}{
		{name: "no user in context", user: nil, wantStatus: http.StatusForbidden},
		{
			name:       "blocked user",
			user:       &models.User{Metadata: models.UserMetadata{Blocked: true, PoliciesAccepted: true}},
			wantStatus: http.StatusForbidden,
		},
		{
			name:         "policies not accepted",
			user:         &models.User{Metadata: models.UserMetadata{PoliciesAccepted: false}},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/account-issue",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled := false
			h := middlewares.RequireValidUser(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true }),
			)

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			if tc.user != nil {
				req = req.WithContext(models.UserToCtx(req.Context(), tc.user))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tc.wantLocation {
				t.Errorf("Location = %q, want %q", got, tc.wantLocation)
			}
			if nextCalled {
				t.Error("next handler must not run")
			}
		})
	}
}
