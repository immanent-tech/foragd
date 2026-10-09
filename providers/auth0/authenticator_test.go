/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package auth0

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	testIssuer   = "https://tenant.example.com/"
	testClientID = "test-client"
)

// ---------------------------------------------------------------------------------------------------------------------
// Fakes and helpers
// ---------------------------------------------------------------------------------------------------------------------

type sessionCtxKey struct{}

// sessionData is one user's session. Like SCS, it travels in the request context, so every simulated request gets its
// own independent session.
type sessionData struct {
	mu     sync.Mutex
	values map[string]any
}

func (d *sessionData) get(key string) any {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.values[key]
}

func (d *sessionData) str(key string) string {
	s, _ := d.get(key).(string)
	return s
}

func (d *sessionData) has(key string) bool { return d.get(key) != nil }

// newSession creates a session pre-populated with vals and returns a context carrying it.
func newSession(vals map[string]any) (context.Context, *sessionData) {
	d := &sessionData{values: map[string]any{}}
	maps.Copy(d.values, vals)
	return context.WithValue(context.Background(), sessionCtxKey{}, d), d
}

// fakeSessions implements SessionManager on top of the per-context sessionData.
type fakeSessions struct {
	renewed  atomic.Int32
	renewErr error
}

func (f *fakeSessions) data(ctx context.Context) *sessionData {
	d, ok := ctx.Value(sessionCtxKey{}).(*sessionData)
	if !ok {
		panic("fakeSessions: no session in context, create one with newSession")
	}
	return d
}

func (f *fakeSessions) Get(ctx context.Context, key string) any { return f.data(ctx).get(key) }

func (f *fakeSessions) Put(ctx context.Context, key string, value any) {
	d := f.data(ctx)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values[key] = value
}

func (f *fakeSessions) Remove(ctx context.Context, key string) {
	d := f.data(ctx)
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.values, key)
}

func (f *fakeSessions) RenewToken(context.Context) error {
	f.renewed.Add(1)
	return f.renewErr
}

// newTestAuthenticator builds an Authenticator directly (no discovery, no config globals) pointed at tokenURL.
func newTestAuthenticator(tokenURL string) (*Authenticator, *fakeSessions) {
	sessions := &fakeSessions{}
	return &Authenticator{
		oauth: &oauth2.Config{
			ClientID:     testClientID,
			ClientSecret: "secret",
			RedirectURL:  "https://app.example.com/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://tenant.example.com/authorize",
				TokenURL: tokenURL,
				// Fixed style: with auto-detect, oauth2 retries failed requests with the other style, which would
				// double the request counts asserted below.
				AuthStyle: oauth2.AuthStyleInParams,
			},
			Scopes: []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email"},
		},
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		sessionManager: sessions,
		baseURL:        "https://app.example.com",
		config: &Config{
			Domain:   "tenant.example.com",
			ClientID: testClientID,
		},
	}, sessions
}

// tokenServer is a fake Auth0 /oauth/token endpoint that counts requests.
type tokenServer struct {
	*httptest.Server

	hits atomic.Int32
}

func newTokenServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.hits.Add(1)
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func tokenResponse(access, refresh string) map[string]any {
	m := map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600}
	if refresh != "" {
		m["refresh_token"] = refresh
	}
	return m
}

// expiredSession returns a session holding an expired access token and a refresh token.
func expiredSession() (context.Context, *sessionData) {
	return newSession(map[string]any{
		sessionKeyAccessToken:  "at-1",
		sessionKeyRefreshToken: "rt-1",
		sessionKeyTokenExpiry:  time.Now().Add(-time.Minute),
	})
}

func assertExpiresIn(t *testing.T, got any, want time.Duration) {
	t.Helper()
	exp, ok := got.(time.Time)
	if !ok {
		t.Fatalf("token expiry not stored as time.Time: %T", got)
	}
	if remaining := time.Until(exp); remaining < want-time.Minute || remaining > want+time.Minute {
		t.Errorf("token expires in %s, want about %s", remaining, want)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------------------------------------------------

func TestRefreshTokens_Success(t *testing.T) {
	tests := []struct {
		name        string
		respRefresh string // refresh token returned by Auth0 ("" = rotation off)
		wantRefresh string
	}{
		{name: "rotation off keeps existing refresh token", respRefresh: "", wantRefresh: "rt-1"},
		{name: "rotation on stores new refresh token", respRefresh: "rt-2", wantRefresh: "rt-2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.PostForm.Get("grant_type"); got != "refresh_token" {
					t.Errorf("grant_type = %q, want refresh_token", got)
				}
				if got := r.PostForm.Get("refresh_token"); got != "rt-1" {
					t.Errorf("refresh_token = %q, want rt-1", got)
				}
				writeJSON(w, http.StatusOK, tokenResponse("at-2", tc.respRefresh))
			})
			a, _ := newTestAuthenticator(srv.URL)
			ctx, sess := expiredSession()

			if err := a.refreshTokens(ctx); err != nil {
				t.Fatalf("RefreshTokens() error = %v", err)
			}
			if got := sess.str(sessionKeyAccessToken); got != "at-2" {
				t.Errorf("access token = %q, want at-2", got)
			}
			if got := sess.str(sessionKeyRefreshToken); got != tc.wantRefresh {
				t.Errorf("refresh token = %q, want %q", got, tc.wantRefresh)
			}
			assertExpiresIn(t, sess.get(sessionKeyTokenExpiry), time.Hour)
			if a.IsAccessTokenExpired(ctx) {
				t.Error("access token should not be expired after refresh")
			}
		})
	}
}

func TestRefreshTokens_InvalidGrantEndsSession(t *testing.T) {
	srv := newTokenServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":             "invalid_grant",
			"error_description": "Unknown or invalid refresh token.",
		})
	})
	a, _ := newTestAuthenticator(srv.URL)
	ctx, sess := expiredSession()

	err := a.refreshTokens(ctx)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("error = %v, want ErrSessionExpired", err)
	}
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		t.Error("underlying *oauth2.RetrieveError should remain reachable for logging")
	}
	for _, key := range []string{sessionKeyAccessToken, sessionKeyRefreshToken, sessionKeyTokenExpiry} {
		if sess.has(key) {
			t.Errorf("session key %q should be cleared after invalid_grant", key)
		}
	}
	if a.IsAuthenticated(ctx) {
		t.Error("IsAuthenticated() = true after invalid_grant")
	}
}

func TestRefreshTokens_TransientErrorKeepsSession(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T) string // returns token URL
	}{
		{
			name: "auth0 returns 500",
			build: func(t *testing.T) string {
				return newTokenServer(t, func(w http.ResponseWriter, _ *http.Request) {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
				}).URL
			},
		},
		{
			name: "auth0 unreachable",
			build: func(t *testing.T) string {
				srv := newTokenServer(t, func(http.ResponseWriter, *http.Request) {})
				srv.Close()
				return srv.URL
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestAuthenticator(tc.build(t))
			ctx, sess := expiredSession()

			err := a.refreshTokens(ctx)
			if err == nil {
				t.Fatal("RefreshTokens() error = nil, want error")
			}
			if errors.Is(err, ErrSessionExpired) {
				t.Errorf("transient failure must not be reported as ErrSessionExpired: %v", err)
			}
			if sess.str(sessionKeyAccessToken) != "at-1" || sess.str(sessionKeyRefreshToken) != "rt-1" {
				t.Error("session tokens must be left untouched after a transient failure")
			}
		})
	}
}

func TestRefreshTokens_NoRefreshToken(t *testing.T) {
	tests := []struct {
		name string
		vals map[string]any
	}{
		{"refresh token missing", map[string]any{sessionKeyAccessToken: "at-1"}},
		{"refresh token empty", map[string]any{sessionKeyAccessToken: "at-1", sessionKeyRefreshToken: ""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTokenServer(t, func(http.ResponseWriter, *http.Request) {
				t.Error("token endpoint must not be called without a refresh token")
			})
			a, _ := newTestAuthenticator(srv.URL)
			ctx, _ := newSession(tc.vals)

			if err := a.refreshTokens(ctx); !errors.Is(err, ErrSessionExpired) {
				t.Fatalf("error = %v, want ErrSessionExpired", err)
			}
			// Regression: a stale access token left behind makes /login redirect to /home in a loop.
			if a.IsAuthenticated(ctx) {
				t.Error("stale access token must be cleared so IsAuthenticated() is false")
			}
		})
	}
}

func TestRefreshTokens_ConcurrentRequestsShareOneRefresh(t *testing.T) {
	const requests = 20

	release := make(chan struct{})
	entered := make(chan struct{}, requests)
	srv := newTokenServer(t, func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writeJSON(w, http.StatusOK, tokenResponse("at-2", "rt-2")) // rotation on
	})
	a, _ := newTestAuthenticator(srv.URL)

	ctxs := make([]context.Context, requests)
	sessions := make([]*sessionData, requests)
	errs := make([]error, requests)
	for i := range requests {
		ctxs[i], sessions[i] = expiredSession() // each request has its own copy of the session
	}

	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = a.refreshTokens(ctxs[i])
		}(i)
	}

	<-entered                          // first request reached Auth0
	time.Sleep(100 * time.Millisecond) // let the rest join the in-flight call
	close(release)
	wg.Wait()

	// With rotation on, a second use of rt-1 would make Auth0 revoke the whole token family.
	if got := srv.hits.Load(); got != 1 {
		t.Errorf("token endpoint hits = %d, want 1", got)
	}
	for i := range requests {
		if errs[i] != nil {
			t.Errorf("request %d: error = %v", i, errs[i])
			continue
		}
		if sessions[i].str(sessionKeyAccessToken) != "at-2" || sessions[i].str(sessionKeyRefreshToken) != "rt-2" {
			t.Errorf("request %d: session not updated with refreshed tokens", i)
		}
	}
}

func TestEnsureFresh(t *testing.T) {
	newServer := func(t *testing.T) *tokenServer {
		return newTokenServer(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, tokenResponse("at-2", ""))
		})
	}

	t.Run("not authenticated", func(t *testing.T) {
		srv := newServer(t)
		a, _ := newTestAuthenticator(srv.URL)
		ctx, _ := newSession(nil)
		if err := a.EnsureFresh(ctx); !errors.Is(err, ErrSessionExpired) {
			t.Errorf("error = %v, want ErrSessionExpired", err)
		}
		if srv.hits.Load() != 0 {
			t.Error("token endpoint should not be called")
		}
	})

	t.Run("fresh token is left alone", func(t *testing.T) {
		srv := newServer(t)
		a, _ := newTestAuthenticator(srv.URL)
		ctx, sess := newSession(map[string]any{
			sessionKeyAccessToken:  "at-1",
			sessionKeyRefreshToken: "rt-1",
			sessionKeyTokenExpiry:  time.Now().Add(time.Hour),
		})
		if err := a.EnsureFresh(ctx); err != nil {
			t.Fatalf("EnsureFresh() error = %v", err)
		}
		if srv.hits.Load() != 0 || sess.str(sessionKeyAccessToken) != "at-1" {
			t.Error("a fresh token must not be refreshed")
		}
	})

	t.Run("token inside the leeway window is refreshed", func(t *testing.T) {
		srv := newServer(t)
		a, _ := newTestAuthenticator(srv.URL)
		ctx, sess := newSession(map[string]any{
			sessionKeyAccessToken:  "at-1",
			sessionKeyRefreshToken: "rt-1",
			sessionKeyTokenExpiry:  time.Now().Add(expiryLeeway / 3),
		})
		if err := a.EnsureFresh(ctx); err != nil {
			t.Fatalf("EnsureFresh() error = %v", err)
		}
		if srv.hits.Load() != 1 || sess.str(sessionKeyAccessToken) != "at-2" {
			t.Error("a token about to expire should be refreshed")
		}
	})

	t.Run("expired token is refreshed", func(t *testing.T) {
		srv := newServer(t)
		a, _ := newTestAuthenticator(srv.URL)
		ctx, sess := expiredSession()
		if err := a.EnsureFresh(ctx); err != nil {
			t.Fatalf("EnsureFresh() error = %v", err)
		}
		if sess.str(sessionKeyAccessToken) != "at-2" {
			t.Error("expired token should be replaced")
		}
	})
}

func TestIsAccessTokenExpired(t *testing.T) {
	tests := []struct {
		name   string
		expiry any // nil = unset
		want   bool
	}{
		{"unset", nil, true},
		{"zero time", time.Time{}, true},
		{"wrong type", "tomorrow", true},
		{"in the past", time.Now().Add(-time.Hour), true},
		{"inside leeway", time.Now().Add(expiryLeeway / 2), true},
		{"in the future", time.Now().Add(time.Hour), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestAuthenticator("http://unused.invalid")
			vals := map[string]any{}
			if tc.expiry != nil {
				vals[sessionKeyTokenExpiry] = tc.expiry
			}
			ctx, _ := newSession(vals)
			if got := a.IsAccessTokenExpired(ctx); got != tc.want {
				t.Errorf("IsAccessTokenExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSaveTokens(t *testing.T) {
	t.Run("empty refresh token does not overwrite the stored one", func(t *testing.T) {
		a, _ := newTestAuthenticator("http://unused.invalid")
		ctx, sess := newSession(map[string]any{sessionKeyRefreshToken: "rt-1"})
		a.saveTokens(ctx, &oauth2.Token{AccessToken: "at", Expiry: time.Now().Add(time.Hour)})
		if got := sess.str(sessionKeyRefreshToken); got != "rt-1" {
			t.Errorf("refresh token = %q, want rt-1", got)
		}
	})

	t.Run("missing expiry falls back to a default lifetime", func(t *testing.T) {
		a, _ := newTestAuthenticator("http://unused.invalid")
		ctx, sess := newSession(nil)
		a.saveTokens(ctx, &oauth2.Token{AccessToken: "at"})
		assertExpiresIn(t, sess.get(sessionKeyTokenExpiry), defaultTokenLifetime)
		if a.IsAccessTokenExpired(ctx) {
			t.Error("a token with no expiry must not be treated as expired on every request")
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------
// Login flow: auth URL, state, return_to, logout
// ---------------------------------------------------------------------------------------------------------------------

func TestGenerateAuthURL(t *testing.T) {
	tests := []struct {
		name     string
		signup   bool
		audience string
	}{
		{"login", false, ""},
		{"login with audience", false, "https://api.example.com"},
		{"signup", true, ""},
		{"signup with audience", true, "https://api.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestAuthenticator("http://unused.invalid")
			a.audience = tc.audience

			res, err := a.GenerateAuthURL(tc.signup)
			if err != nil {
				t.Fatalf("GenerateAuthURL() error = %v", err)
			}
			u, err := url.Parse(res.URL)
			if err != nil {
				t.Fatalf("parse url: %v", err)
			}
			q := u.Query()

			want := map[string]string{
				"response_type":         "code",
				"client_id":             testClientID,
				"redirect_uri":          "https://app.example.com/callback",
				"state":                 res.State,
				"nonce":                 res.Nonce,
				"code_challenge_method": "S256",
				"code_challenge":        oauth2.S256ChallengeFromVerifier(res.CodeVerifier),
			}
			for k, v := range want {
				if got := q.Get(k); got != v || v == "" {
					t.Errorf("query %s = %q, want %q", k, got, v)
				}
			}
			scopes := strings.Fields(q.Get("scope"))
			for _, s := range []string{"openid", "offline_access"} {
				if !slices.Contains(scopes, s) {
					t.Errorf("scope %q missing from %v", s, scopes)
				}
			}
			if got := q.Get("screen_hint"); (got == "signup") != tc.signup {
				t.Errorf("screen_hint = %q, signup = %v", got, tc.signup)
			}
			if got := q.Get("audience"); got != tc.audience {
				t.Errorf("audience = %q, want %q", got, tc.audience)
			}
			if strings.Contains(res.URL, res.CodeVerifier) {
				t.Error("the raw PKCE verifier must never appear in the authorization URL")
			}
		})
	}
}

func TestGenerateAuthURL_ValuesAreRandom(t *testing.T) {
	a, _ := newTestAuthenticator("http://unused.invalid")
	first, err := a.GenerateAuthURL(false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.GenerateAuthURL(false)
	if err != nil {
		t.Fatal(err)
	}
	if first.State == second.State || first.Nonce == second.Nonce || first.CodeVerifier == second.CodeVerifier {
		t.Error("state, nonce and verifier must differ between requests")
	}
	if len(first.State) < 43 || len(first.Nonce) < 43 {
		t.Errorf("state/nonce too short: %d/%d chars", len(first.State), len(first.Nonce))
	}
}

func TestStoreAuthRequestAndValidateState(t *testing.T) {
	a, _ := newTestAuthenticator("http://unused.invalid")
	ctx, _ := newSession(nil)
	res := &AuthURLResult{State: "state-1", Nonce: "nonce-1", CodeVerifier: "verifier-1"}

	if err := a.ValidateState(ctx, "state-1"); err == nil {
		t.Error("ValidateState() should fail before anything is stored")
	}

	a.StoreAuthRequest(ctx, res)

	if got, _ := a.getNonce(ctx); got != "nonce-1" {
		t.Errorf("nonce = %q", got)
	}
	if got, _ := a.getCodeVerifier(ctx); got != "verifier-1" {
		t.Errorf("verifier = %q", got)
	}
	if err := a.ValidateState(ctx, "state-1"); err != nil {
		t.Errorf("ValidateState(match) error = %v", err)
	}
	if err := a.ValidateState(ctx, "other"); !errors.Is(err, ErrInvalidState) {
		t.Errorf("ValidateState(mismatch) error = %v, want ErrInvalidState", err)
	}
	if err := a.ValidateState(ctx, ""); !errors.Is(err, ErrInvalidState) {
		t.Errorf("ValidateState(empty) error = %v, want ErrInvalidState", err)
	}

	a.clearState(ctx)
	if _, err := a.getState(ctx); err == nil {
		t.Error("state should be cleared by ClearState")
	}
}

func TestSafeReturnTo(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/home", "/home"},
		{"/articles/42?page=2&x=y", "/articles/42?page=2&x=y"},
		{"", "/"},
		{"home", "/"},
		{"//evil.com", "/"},
		{"//evil.com/path", "/"},
		{`/\evil.com`, "/"},
		{`/\\evil.com`, "/"},
		{"https://evil.com/home", "/"},
		{"javascript:alert(1)", "/"},
		{"/ok\r\nSet-Cookie: x=y", "/"},
	}
	for _, tc := range tests {
		if got := safeReturnTo(tc.in); got != tc.want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPutReturnTo(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/home", "/home"},
		{"/home/updates", "/home"},
		{"/home/paginate", "/home"},
		{"/home/paginate?page=2", "/home?page=2"},
		{"/updates", "/"},
		{"//evil.com", "/"},
		{"https://evil.com/x", "/"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			a, _ := newTestAuthenticator("http://unused.invalid")
			ctx, _ := newSession(nil)
			a.PutReturnTo(ctx, tc.in)
			got, err := a.getReturnTo(ctx)
			if err != nil {
				t.Fatalf("GetReturnTo() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("return_to = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetReturnTo_RevalidatesStoredValue(t *testing.T) {
	a, _ := newTestAuthenticator("http://unused.invalid")
	ctx, _ := newSession(map[string]any{sessionKeyReturnTo: "//evil.com"}) // e.g. written by other code
	got, err := a.getReturnTo(ctx)
	if err != nil || got != "/" {
		t.Errorf("GetReturnTo() = %q, %v; want \"/\", nil", got, err)
	}
}

func TestConsumeReturnTo_IsSingleUse(t *testing.T) {
	a, _ := newTestAuthenticator("http://unused.invalid")
	ctx, _ := newSession(nil)
	a.PutReturnTo(ctx, "/articles/42")

	got, err := a.ConsumeReturnTo(ctx)
	if err != nil || got != "/articles/42" {
		t.Fatalf("first ConsumeReturnTo() = %q, %v", got, err)
	}
	if _, err := a.ConsumeReturnTo(ctx); err == nil {
		t.Error("second ConsumeReturnTo() should fail: the value must be single-use")
	}
}

func TestGenerateLogoutURL(t *testing.T) {
	a, _ := newTestAuthenticator("http://unused.invalid")
	u, err := a.GenerateLogoutURL()
	if err != nil {
		t.Fatalf("GenerateLogoutURL() error = %v", err)
	}
	if u.Scheme != "https" || u.Host != "tenant.example.com" || u.Path != "/v2/logout" {
		t.Errorf("unexpected logout endpoint: %s", u)
	}
	q := u.Query()
	if q.Get("client_id") != testClientID {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	// Must come from configuration, never from request headers.
	if q.Get("returnTo") != "https://app.example.com" {
		t.Errorf("returnTo = %q", q.Get("returnTo"))
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Code exchange
// ---------------------------------------------------------------------------------------------------------------------

var testKey = sync.OnceValue(func() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
})

// signIDToken builds an RS256-signed JWT without any JOSE library.
func signIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	input := enc(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "test-key"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, testKey(), crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

type exchangeScenario struct {
	tokenNonce   string        // nonce inside the ID token; "" = the nonce stored in the session
	audience     string        // "" = the client ID
	idExpiry     time.Duration // 0 = valid for an hour
	omitIDToken  bool
	omitRefresh  bool
	clearVerifer bool // simulate a session without the PKCE verifier
	renewErr     error
}

type exchangeFixture struct {
	auth     *Authenticator
	sessions *fakeSessions
	srv      *tokenServer
	ctx      context.Context
	sess     *sessionData
}

const (
	exchangeCode  = "auth-code"
	exchangeNonce = "nonce-1"
)

func newExchangeFixture(t *testing.T, sc exchangeScenario) *exchangeFixture {
	t.Helper()
	verifier := oauth2.GenerateVerifier()

	srv := newTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.PostForm.Get("grant_type") != "authorization_code",
			r.PostForm.Get("code") != exchangeCode,
			r.PostForm.Get("code_verifier") != verifier, // PKCE must actually be sent
			r.PostForm.Get("redirect_uri") != "https://app.example.com/callback":
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}

		resp := tokenResponse("at-1", "rt-1")
		if sc.omitRefresh {
			delete(resp, "refresh_token")
		}
		if !sc.omitIDToken {
			nonce, aud, life := sc.tokenNonce, sc.audience, sc.idExpiry
			if nonce == "" {
				nonce = exchangeNonce
			}
			if aud == "" {
				aud = testClientID
			}
			if life == 0 {
				life = time.Hour
			}
			resp["id_token"] = signIDToken(t, map[string]any{
				"iss":   testIssuer,
				"sub":   "auth0|123",
				"aud":   aud,
				"nonce": nonce,
				"email": "user@example.com",
				"iat":   time.Now().Add(-time.Minute).Unix(),
				"exp":   time.Now().Add(life).Unix(),
			})
		}
		writeJSON(w, http.StatusOK, resp)
	})

	a, sessions := newTestAuthenticator(srv.URL)
	sessions.renewErr = sc.renewErr
	a.verifier = oidc.NewVerifier(testIssuer,
		&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&testKey().PublicKey}},
		&oidc.Config{ClientID: testClientID},
	)

	vals := map[string]any{
		sessionKeyState:        "state-1",
		sessionKeyNonce:        exchangeNonce,
		sessionKeyCodeVerifier: verifier,
	}
	if sc.clearVerifer {
		delete(vals, sessionKeyCodeVerifier)
	}
	ctx, sess := newSession(vals)
	return &exchangeFixture{auth: a, sessions: sessions, srv: srv, ctx: ctx, sess: sess}
}

func TestPerformExchange_Success(t *testing.T) {
	f := newExchangeFixture(t, exchangeScenario{})

	profile, err := f.auth.PerformExchange(f.ctx, exchangeCode)
	if err != nil {
		t.Fatalf("PerformExchange() error = %v", err)
	}
	if profile == nil {
		t.Fatal("profile = nil")
	}
	if got := f.sessions.renewed.Load(); got != 1 {
		t.Errorf("RenewToken calls = %d, want 1 (session fixation protection)", got)
	}
	if f.sess.str(sessionKeyAccessToken) != "at-1" || f.sess.str(sessionKeyRefreshToken) != "rt-1" {
		t.Error("tokens were not saved to the session")
	}
	assertExpiresIn(t, f.sess.get(sessionKeyTokenExpiry), time.Hour)
	for _, key := range []string{sessionKeyState, sessionKeyNonce, sessionKeyCodeVerifier} {
		if f.sess.has(key) {
			t.Errorf("single-use key %q should be cleared after the exchange", key)
		}
	}
}

func TestPerformExchange_NoRefreshTokenIssued(t *testing.T) {
	f := newExchangeFixture(t, exchangeScenario{omitRefresh: true})

	if _, err := f.auth.PerformExchange(f.ctx, exchangeCode); err != nil {
		t.Fatalf("PerformExchange() error = %v (login should still succeed)", err)
	}
	if f.sess.has(sessionKeyRefreshToken) {
		t.Error("no refresh token should be stored when Auth0 did not issue one")
	}
	if f.sess.str(sessionKeyAccessToken) != "at-1" {
		t.Error("access token should still be saved")
	}
}

func TestPerformExchange_Failures(t *testing.T) {
	tests := []struct {
		name         string
		scenario     exchangeScenario
		wantErr      error // nil = any error
		wantTokenHit bool
		wantRenewed  int32
	}{
		{
			name:         "nonce mismatch",
			scenario:     exchangeScenario{tokenNonce: "attacker-nonce"},
			wantErr:      ErrInvalidToken,
			wantTokenHit: true,
		},
		{
			name:         "no id_token in response",
			scenario:     exchangeScenario{omitIDToken: true},
			wantErr:      ErrNoIDToken,
			wantTokenHit: true,
		},
		{
			name:         "id token for another client",
			scenario:     exchangeScenario{audience: "someone-else"},
			wantTokenHit: true,
		},
		{
			name:         "expired id token",
			scenario:     exchangeScenario{idExpiry: -time.Hour},
			wantTokenHit: true,
		},
		{
			name:         "session renewal fails",
			scenario:     exchangeScenario{renewErr: errors.New("store unavailable")},
			wantTokenHit: true,
			wantRenewed:  1,
		},
		{
			name:     "missing PKCE verifier in session",
			scenario: exchangeScenario{clearVerifer: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newExchangeFixture(t, tc.scenario)

			profile, err := f.auth.PerformExchange(f.ctx, exchangeCode)
			if err == nil {
				t.Fatal("PerformExchange() error = nil, want error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
			if profile != nil {
				t.Error("profile must be nil on failure")
			}
			if f.sess.has(sessionKeyAccessToken) || f.sess.has(sessionKeyRefreshToken) {
				t.Error("no tokens may be saved when the exchange fails")
			}
			if got := f.srv.hits.Load() > 0; got != tc.wantTokenHit {
				t.Errorf("token endpoint called = %v, want %v", got, tc.wantTokenHit)
			}
			if got := f.sessions.renewed.Load(); got != tc.wantRenewed {
				t.Errorf("RenewToken calls = %d, want %d", got, tc.wantRenewed)
			}
			// State, nonce and verifier are single-use even on failure, so a callback can't be replayed.
			for _, key := range []string{sessionKeyState, sessionKeyNonce, sessionKeyCodeVerifier} {
				if f.sess.has(key) {
					t.Errorf("single-use key %q should be cleared even after a failure", key)
				}
			}
		})
	}
}

func TestPerformExchange_WrongCodeIsRejectedByAuth0(t *testing.T) {
	f := newExchangeFixture(t, exchangeScenario{})
	if _, err := f.auth.PerformExchange(f.ctx, "forged-code"); err == nil {
		t.Fatal("PerformExchange() error = nil, want error")
	}
	if f.sess.has(sessionKeyAccessToken) {
		t.Error("no tokens may be saved when Auth0 rejects the code")
	}
}
