/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package auth0

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/session"
)

// Session key constants used to store values in the SCS session.
const (
	sessionKeyAccessToken  = "access_token"
	sessionKeyRefreshToken = "refresh_token"
	sessionKeyTokenExpiry  = "token_expiry"
	sessionKeyState        = "oauth_state"
	sessionKeyNonce        = "oauth_nonce"
	sessionKeyCodeVerifier = "pkce_code_verifier"
	sessionKeyReturnTo     = "return_to"
)

const (
	// expiryLeeway is how early before the real expiry we treat the access token as expired. It covers clock skew and
	// in-flight request time.
	expiryLeeway = 30 * time.Second
	// defaultTokenLifetime is used only if the token response carries no expiry at all.
	defaultTokenLifetime = 10 * time.Minute
	discoveryTimeout     = 15 * time.Second
	refreshTimeout       = 15 * time.Second
	randomBytes          = 32
	loginPath            = "/login"
)

// SessionManager is the subset of the SCS session manager used by the authenticator.
type SessionManager interface {
	Get(ctx context.Context, key string) any
	Put(ctx context.Context, key string, value any)
	Remove(ctx context.Context, key string)
	RenewToken(ctx context.Context) error
}

var (
	ErrNoIDToken      = errors.New("no id_token field in oauth2 token")
	ErrInvalidToken   = errors.New("token is invalid")
	ErrInvalidState   = errors.New("oauth state does not match")
	ErrSessionExpired = errors.New("session expired, re-authentication required")
)

// Authenticator is used to authenticate our users.
type Authenticator struct {
	provider       *oidc.Provider
	oauth          *oauth2.Config
	verifier       *oidc.IDTokenVerifier
	httpClient     *http.Client
	sessionManager SessionManager
	// baseURL is the scheme://host of this application, derived from the configured callback URL. It is used for
	// post-logout redirects instead of trusting the request's Host header.
	baseURL string
	// audience is optional. Set it to get a JWT access token for your API instead of an opaque /userinfo token.
	audience string

	// refreshGroup collapses concurrent refreshes of the same refresh token into one upstream call. This matters when
	// refresh token rotation is on: reusing a rotated token revokes the whole token family.
	refreshGroup singleflight.Group

	config *Config
}

var (
	loadMu sync.Mutex
	loaded *Authenticator
)

// LoadAuthenticator performs setup and initialisation of the Auth0 tenant. It can be called repeatedly and lazily:
// only a successful initialisation is cached, so a transient discovery failure is retried on the next call rather than
// being stuck until restart.
func LoadAuthenticator() (*Authenticator, error) {
	loadMu.Lock()
	defer loadMu.Unlock()

	if loaded != nil {
		return loaded, nil
	}
	a, err := newAuthenticator()
	if err != nil {
		return nil, err
	}
	loaded = a
	return a, nil
}

func newAuthenticator() (*Authenticator, error) {
	cfg, err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	callback, err := url.Parse(cfg.CallbackURL)
	if err != nil || callback.Scheme == "" || callback.Host == "" {
		return nil, fmt.Errorf("invalid callback url %q", cfg.CallbackURL)
	}
	baseURL := (&url.URL{Scheme: callback.Scheme, Host: callback.Host}).String()

	httpClient := newHTTPClient()

	// Use the hardened client and a timeout for discovery. go-oidc only retains the HTTP client from this context
	// (for later JWKS fetches), not its cancellation, so cancelling after discovery is safe.
	ctx, cancel := context.WithTimeout(oidc.ClientContext(context.Background(), httpClient), discoveryTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, "https://"+cfg.Domain+"/")
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	sessionMgr, err := session.Load()
	if err != nil {
		return nil, fmt.Errorf("load session manager: %w", err)
	}

	return &Authenticator{
		provider: provider,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email"},
		},
		verifier:       provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}), // built once, reused.
		httpClient:     httpClient,
		sessionManager: sessionMgr,
		baseURL:        baseURL,
		audience:       cfg.Audience,
		config:         cfg,
	}, nil
}

// clientCtx attaches the hardened HTTP client so oauth2 and go-oidc use it for their requests.
func (a *Authenticator) clientCtx(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, a.httpClient)
}

// ---------------------------------------------------------------------------------------------------------------------
// Login flow
// ---------------------------------------------------------------------------------------------------------------------

// AuthURLResult holds the generated authorization URL along with the state, nonce and PKCE code verifier that must be
// stored in the session before redirecting (see StoreAuthRequest).
type AuthURLResult struct {
	URL          string
	State        string
	Nonce        string
	CodeVerifier string
}

func (a AuthURLResult) GetURL() string          { return a.URL }
func (a AuthURLResult) GetState() string        { return a.State }
func (a AuthURLResult) GetNonce() string        { return a.Nonce }
func (a AuthURLResult) GetCodeVerifier() string { return a.CodeVerifier }

// GenerateAuthURL constructs the Auth0 Universal Login redirect URL using PKCE (S256), state and a nonce. Pass
// signup=true to show the signup screen first. The caller decides this (e.g. from the route), so there is no hidden
// dependency on the router and no way to get an empty URL back.
func (a *Authenticator) GenerateAuthURL(signup bool) (*AuthURLResult, error) {
	state, err := randomString(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	nonce, err := randomString(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	verifier := oauth2.GenerateVerifier()

	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier), // takes the verifier and derives the challenge itself.
		oidc.Nonce(nonce),
	}
	if signup {
		opts = append(opts, oauth2.SetAuthURLParam("screen_hint", "signup"))
	}
	if a.audience != "" {
		opts = append(opts, oauth2.SetAuthURLParam("audience", a.audience))
	}

	return &AuthURLResult{
		URL:          a.oauth.AuthCodeURL(state, opts...),
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
	}, nil
}

// StoreAuthRequest saves everything the callback needs to verify the response. Call it before redirecting.
func (a *Authenticator) StoreAuthRequest(ctx context.Context, r *AuthURLResult) {
	a.putState(ctx, r.State)
	a.putNonce(ctx, r.Nonce)
	a.putCodeVerifier(ctx, r.CodeVerifier)
}

// ValidateState compares the state returned on the callback with the one stored in the session.
func (a *Authenticator) ValidateState(ctx context.Context, got string) error {
	want, err := a.getState(ctx)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		return ErrInvalidState
	}
	return nil
}

// PerformExchange verifies the callback and exchanges the authorization code for tokens. The PKCE verifier and nonce
// are read from the session (stored by StoreAuthRequest). Call ValidateState first. State, nonce and verifier are
// single-use and are cleared whether or not the exchange succeeds.
func (a *Authenticator) PerformExchange(ctx context.Context, code string) (*models.UserProfileResponse, error) {
	defer a.clearState(ctx)

	verifier, err := a.getCodeVerifier(ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := a.getNonce(ctx)
	if err != nil {
		return nil, err
	}

	token, err := a.oauth.Exchange(a.clientCtx(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange code for token: %w", err)
	}

	idToken, err := a.verifyIDToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrInvalidToken)
	}

	var profile models.UserProfileResponse
	if err = idToken.Claims(&profile); err != nil {
		return nil, fmt.Errorf("extract user profile: %w", err)
	}

	if token.RefreshToken == "" {
		// Needs offline_access (requested) and, usually, an audience whose API has "Allow Offline Access" enabled.
		slog.WarnContext(ctx, "auth0 did not issue a refresh token; session will end when the access token expires")
	}

	// Rotate the session ID on privilege change to prevent session fixation.
	if err = a.sessionManager.RenewToken(ctx); err != nil {
		return nil, fmt.Errorf("renew session: %w", err)
	}
	a.saveTokens(ctx, token)

	return &profile, nil
}

// GenerateLogoutURL generates the URL that logs the user out of Auth0. The return URL comes from configuration, not
// from the request's Host header.
func (a *Authenticator) GenerateLogoutURL() (*url.URL, error) {
	logoutURL, err := url.Parse("https://" + a.config.Domain + "/v2/logout")
	if err != nil {
		return nil, fmt.Errorf("generate logout url: %w", err)
	}
	params := url.Values{}
	params.Set("returnTo", a.baseURL)
	params.Set("client_id", a.config.ClientID)
	logoutURL.RawQuery = params.Encode()

	return logoutURL, nil
}

// verifyIDToken verifies that an *oauth2.Token carries a valid ID token and returns it.
func (a *Authenticator) verifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, ErrNoIDToken
	}
	id, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("unable to verify token: %w", err)
	}
	return id, nil
}

// ---------------------------------------------------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------------------------------------------------

// refreshTokens uses the session's refresh token to obtain new tokens and saves them into the session.
//
// It returns ErrSessionExpired when the user must log in again (no refresh token, or Auth0 rejected it with
// invalid_grant, in which case the session's auth data is also cleared). Any other error is transient (network, 5xx)
// and the caller may retry; the session is left untouched.
func (a *Authenticator) refreshTokens(ctx context.Context) error {
	rt, err := a.getRefreshToken(ctx)
	if err != nil || rt == "" {
		// No way to refresh, so don't leave a stale access token behind. Otherwise IsAuthenticated keeps returning true
		// and /login bounces the user back to /home in a loop.
		a.ClearAuth(ctx)
		return ErrSessionExpired
	}

	v, err, _ := a.refreshGroup.Do(rt, func() (any, error) {
		// Detach from the first caller's cancellation so one client disconnecting doesn't fail every waiter.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
		defer cancel()
		rctx = a.clientCtx(rctx)

		// With only a refresh token set, the TokenSource performs a refresh_token grant. oauth2 also carries the
		// old refresh token forward if the response omits one (rotation off).
		return a.oauth.TokenSource(rctx, &oauth2.Token{RefreshToken: rt}).Token()
	})
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && re.ErrorCode == "invalid_grant" {
			a.ClearAuth(ctx)
			return fmt.Errorf("%w: %w", ErrSessionExpired, err)
		}
		return fmt.Errorf("refresh tokens: %w", err)
	}

	// Each caller saves into its own session context, which is why the singleflight returns the token instead of
	// saving inside the group.
	a.saveTokens(ctx, v.(*oauth2.Token))
	return nil
}

// EnsureFresh guarantees the session holds a usable access token, refreshing it if needed. It returns
// ErrSessionExpired if the user must log in again.
func (a *Authenticator) EnsureFresh(ctx context.Context) error {
	if !a.IsAuthenticated(ctx) {
		return ErrSessionExpired
	}
	if !a.IsAccessTokenExpired(ctx) {
		return nil
	}
	return a.refreshTokens(ctx)
}

// ---------------------------------------------------------------------------------------------------------------------
// Session accessors
// ---------------------------------------------------------------------------------------------------------------------

func (a *Authenticator) putState(ctx context.Context, state string) {
	a.sessionManager.Put(ctx, sessionKeyState, state)
}

func (a *Authenticator) putNonce(ctx context.Context, nonce string) {
	a.sessionManager.Put(ctx, sessionKeyNonce, nonce)
}

func (a *Authenticator) putCodeVerifier(ctx context.Context, verifier string) {
	a.sessionManager.Put(ctx, sessionKeyCodeVerifier, verifier)
}

// PutReturnTo stores the path to return to after login. Only local paths are accepted (open-redirect protection), and
// when triggered from htmx updates/paginate endpoints it returns to the base page.
func (a *Authenticator) PutReturnTo(ctx context.Context, path string) {
	u, err := url.Parse(safeReturnTo(path))
	if err != nil {
		u = &url.URL{Path: "/"}
	}
	switch {
	case strings.HasSuffix(u.Path, "/updates"):
		u.Path = strings.TrimSuffix(u.Path, "/updates")
	case strings.HasSuffix(u.Path, "/paginate"):
		u.Path = strings.TrimSuffix(u.Path, "/paginate")
	}
	a.sessionManager.Put(ctx, sessionKeyReturnTo, u.RequestURI())
}

// ConsumeReturnTo returns the stored return path and removes it (single use).
func (a *Authenticator) ConsumeReturnTo(ctx context.Context) (string, error) {
	p, err := a.getReturnTo(ctx)
	if err != nil {
		return "", err
	}
	a.sessionManager.Remove(ctx, sessionKeyReturnTo)
	return p, nil
}

func (a *Authenticator) getState(ctx context.Context) (string, error) {
	return a.getString(ctx, sessionKeyState, "state")
}

func (a *Authenticator) getNonce(ctx context.Context) (string, error) {
	return a.getString(ctx, sessionKeyNonce, "nonce")
}

func (a *Authenticator) getCodeVerifier(ctx context.Context) (string, error) {
	return a.getString(ctx, sessionKeyCodeVerifier, "code verifier")
}

func (a *Authenticator) getAccessToken(ctx context.Context) (string, error) {
	return a.getString(ctx, sessionKeyAccessToken, "access token")
}

func (a *Authenticator) getRefreshToken(ctx context.Context) (string, error) {
	return a.getString(ctx, sessionKeyRefreshToken, "refresh token")
}

// getReturnTo returns the stored return path, re-validated on the way out as defence in depth.
func (a *Authenticator) getReturnTo(ctx context.Context) (string, error) {
	p, err := a.getString(ctx, sessionKeyReturnTo, "return to")
	if err != nil {
		return "", err
	}
	return safeReturnTo(p), nil
}

// getTokenExpiry returns the real (un-buffered) access token expiry.
func (a *Authenticator) getTokenExpiry(ctx context.Context) (time.Time, error) {
	expiry, ok := a.sessionManager.Get(ctx, sessionKeyTokenExpiry).(time.Time)
	if !ok {
		return time.Time{}, errors.New("no token expiry value in session")
	}
	return expiry, nil
}

func (a *Authenticator) getString(ctx context.Context, key, name string) (string, error) {
	v, ok := a.sessionManager.Get(ctx, key).(string)
	if !ok {
		return "", fmt.Errorf("no %s value in session", name)
	}
	return v, nil
}

// IsAuthenticated returns true if the session contains an access token.
func (a *Authenticator) IsAuthenticated(ctx context.Context) bool {
	tkn, err := a.getAccessToken(ctx)
	return err == nil && tkn != ""
}

// IsAccessTokenExpired returns true if the access token has expired, or will within expiryLeeway.
func (a *Authenticator) IsAccessTokenExpired(ctx context.Context) bool {
	expiry, err := a.getTokenExpiry(ctx)
	if err != nil || expiry.IsZero() {
		return true
	}
	return time.Now().Add(expiryLeeway).After(expiry)
}

// ClearAuth removes all authentication-related keys from the session.
func (a *Authenticator) ClearAuth(ctx context.Context) {
	a.sessionManager.Remove(ctx, sessionKeyAccessToken)
	a.sessionManager.Remove(ctx, sessionKeyRefreshToken)
	a.sessionManager.Remove(ctx, sessionKeyTokenExpiry)
}

// clearState removes all data related to an authorization exchange from the session.
func (a *Authenticator) clearState(ctx context.Context) {
	a.sessionManager.Remove(ctx, sessionKeyState)
	a.sessionManager.Remove(ctx, sessionKeyNonce)
	a.sessionManager.Remove(ctx, sessionKeyCodeVerifier)
}

// saveTokens saves the token data in the session. The refresh token is only overwritten when a new one was actually
// issued: without rotation, Auth0 omits it from refresh responses and blindly writing "" would destroy the stored one.
func (a *Authenticator) saveTokens(ctx context.Context, t *oauth2.Token) {
	expiry := t.Expiry
	if expiry.IsZero() {
		expiry = time.Now().Add(defaultTokenLifetime)
	}
	a.sessionManager.Put(ctx, sessionKeyAccessToken, t.AccessToken)
	a.sessionManager.Put(ctx, sessionKeyTokenExpiry, expiry)
	if t.RefreshToken != "" {
		a.sessionManager.Put(ctx, sessionKeyRefreshToken, t.RefreshToken)
	}
}

// ---------------------------------------------------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------------------------------------------------

// safeReturnTo returns raw only if it is a local path ("/foo?bar"), otherwise "/". It rejects absolute URLs,
// protocol-relative URLs ("//evil.com") and backslash tricks ("/\evil.com").
func safeReturnTo(raw string) string {
	if raw == "" || raw[0] != '/' || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return u.RequestURI()
}

// randomString returns n cryptographically secure random bytes, base64url encoded.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
