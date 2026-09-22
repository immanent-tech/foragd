// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/auth0/go-auth0/v2/authentication"
	"github.com/go-resty/resty/v2"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
)

// Session key constants used to store values in the SCS session.
const (
	sessionKeyAccessToken  = "access_token"
	sessionKeyRefreshToken = "refresh_token"
	sessionKeyTokenExpiry  = "token_expiry"
	sessionKeyUserProfile  = "user_profile"
	sessionKeyState        = "oauth_state"
	sessionKeyCodeVerifier = "pkce_code_verifier"
	sessionKeyReturnTo     = "return_to"
)

type SessionManager interface {
	Get(ctx context.Context, key string) any
	Put(ctx context.Context, key string, value any)
	Remove(ctx context.Context, key string)
}

var ErrNoIDToken = errors.New("no id_token field in oauth2 token")
var ErrInvalidToken = errors.New("token is invalid")

// TokenResponse represents the JSON response from the Auth0 /oauth/token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // seconds until expiry
}

// Authenticator is used to authenticate our users.
type Authenticator struct {
	*oidc.Provider
	oauth2.Config
}

// initAuthenticator will the setup and initialisation of the Auth0 tenant. It can be called multiple times but will
// only perform initialisation once (so it can be lazily loaded by calling it before any Auth0 actions).
var LoadAuthenticator = sync.OnceValues(func() (*Authenticator, error) {
	err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	provider, err := oidc.NewProvider(
		context.Background(),
		"https://"+cfg.Domain+"/",
	)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	conf := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.CallbackURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email"},
	}
	return &Authenticator{
		Provider: provider,
		Config:   conf,
	}, nil
})

// postToken sends a POST request to the Auth0 token endpoint and decodes the response.
func (a *Authenticator) postToken(
	ctx context.Context,
	httpClient *resty.Client,
	form url.Values,
) (*TokenResponse, error) {
	var token TokenResponse
	var errResult authentication.Error

	switch resp, err := httpClient.R().
		SetContext(ctx).
		SetFormDataFromValues(form).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetResult(&token).
		SetError(&errResult).
		Post(a.Config.Endpoint.TokenURL); {
	case err != nil:
		return nil, fmt.Errorf("post token: %w", err)
	case resp.IsError():
		return nil, fmt.Errorf("post token: %d: %s", resp.StatusCode(), resp.Status())
	}

	return &token, nil
}

// Exchange handles verifying and exchanging the authorization code for an access token. It also extracts the ID token
// and user profile.
func (a *Authenticator) PerformExchange(
	ctx context.Context,
	code, verifier string,
) (*TokenResponse, *UserProfile, error) {
	// token, err := AuthClient.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	token, err := a.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code for token: %w", err)
	}

	// Verify token.
	idToken, idTokenHash, err := a.VerifyIDToken(ctx, token)
	if err != nil {
		return nil, nil, fmt.Errorf("verify id token: %w", err)
	}

	// Extract user profile.
	var profile UserProfile
	if err = idToken.Claims(&profile); err != nil {
		return nil, nil, fmt.Errorf("extract user profile: %w", err)
	}

	resp := TokenResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.Type(),
		ExpiresIn:    token.ExpiresIn,
		IDToken:      idTokenHash,
	}

	return &resp, &profile, nil
}

// RefreshTokens exchanges a refresh token for a new set of tokens.
func (a *Authenticator) RefreshTokens(
	ctx context.Context,
	httpClient *resty.Client,
	refreshToken string,
) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.Config.ClientID)
	form.Set("client_secret", a.Config.ClientSecret)
	form.Set("refresh_token", refreshToken)

	return a.postToken(ctx, httpClient, form)
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, string, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, "", ErrNoIDToken
	}
	oidcConfig := &oidc.Config{
		ClientID: a.ClientID,
	}
	id, err := a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", fmt.Errorf("unable to verify token: %w", err)
	}
	return id, rawIDToken, nil
}

// AuthURLResult holds the generated authorization URL along with the state and PKCE code verifier that must be stored
// in the session before redirecting.
type AuthURLResult struct {
	URL          string
	State        string
	CodeVerifier string
}

func (a AuthURLResult) GetURL() string {
	return a.URL
}

func (a AuthURLResult) GetState() string {
	return a.State
}

func (a AuthURLResult) GetCodeVerifier() string {
	return a.CodeVerifier
}

// GenerateAuthURL constructs the Auth0 Universal Login redirect URL using PKCE.
func (a *Authenticator) GenerateAuthURL(req *http.Request) (*AuthURLResult, error) {
	state, err := generateState()
	if err != nil {
		return nil, err
	}

	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, err
	}

	// Redirect the user appropriately.
	var authURL string
	switch chi.RouteContext(req.Context()).RoutePattern() {
	case "/signup":
		// Retrieve and save the selected plan id into the session for later use.
		authURL = a.AuthCodeURL(state,
			oauth2.SetAuthURLParam("screen_hint", "signup"),
			// oauth2.S256ChallengeOption(codeChallenge(verifier)),
		)
	case "/login":
		// authURL = AuthClient.AuthCodeURL(state, oauth2.S256ChallengeOption(codeChallenge(verifier)))
		authURL = a.AuthCodeURL(state)
	}

	return &AuthURLResult{
		URL:          authURL,
		State:        state,
		CodeVerifier: verifier,
	}, nil
}

// GenerateLogoutURL generates URL to log the user out from the auth backend.
func (a *Authenticator) GenerateLogoutURL(req *http.Request) (*url.URL, error) {
	logoutURL, err := url.Parse("https://" + cfg.Domain + "/v2/logout")
	if err != nil {
		return nil, fmt.Errorf("generate logout url: %w", err)
	}

	returnTo, err := url.Parse("https://" + req.Host)
	if err != nil {
		return nil, fmt.Errorf("generate return_to url: %w", err)
	}

	parameters := url.Values{}
	parameters.Add("returnTo", returnTo.String())
	parameters.Add("client_id", cfg.ClientID)
	logoutURL.RawQuery = parameters.Encode()

	return logoutURL, nil
}

func (a *Authenticator) PutState(ctx context.Context, mgr SessionManager, state string) {
	mgr.Put(ctx, sessionKeyState, state)
}

func (a *Authenticator) PutCodeVerifier(ctx context.Context, mgr SessionManager, verifier string) {
	mgr.Put(ctx, sessionKeyCodeVerifier, verifier)
}

func (a *Authenticator) PutReturnTo(ctx context.Context, mgr SessionManager, path string) {
	// When triggered from updates/paginate, return to the base page.
	switch {
	case strings.HasSuffix(path, "/updates"):
		path = strings.TrimSuffix(path, "/updates")
	case strings.HasSuffix(path, "/paginate"):
		path = strings.TrimSuffix(path, "/paginate")
	}
	mgr.Put(ctx, sessionKeyReturnTo, path)
}

func (a *Authenticator) GetState(ctx context.Context, mgr SessionManager) (string, error) {
	state, ok := mgr.Get(ctx, sessionKeyState).(string)
	if !ok {
		return "", errors.New("no state value in session")
	}
	return state, nil
}

func (a *Authenticator) GetCodeVerifier(ctx context.Context, mgr SessionManager) (string, error) {
	code, ok := mgr.Get(ctx, sessionKeyCodeVerifier).(string)
	if !ok {
		return "", errors.New("no code verifier value in session")
	}
	return code, nil
}

func (a *Authenticator) GetAccessToken(ctx context.Context, mgr SessionManager) (string, error) {
	accessToken, ok := mgr.Get(ctx, sessionKeyAccessToken).(string)
	if !ok {
		return "", errors.New("no access token in session")
	}
	return accessToken, nil
}

func (a *Authenticator) GetRefreshToken(ctx context.Context, mgr SessionManager) (string, error) {
	refreshToken, ok := mgr.Get(ctx, sessionKeyRefreshToken).(string)
	if !ok {
		return "", errors.New("no refresh token in session")
	}
	return refreshToken, nil
}

func (a *Authenticator) GetTokenExpiry(ctx context.Context, mgr SessionManager) (time.Time, error) {
	expiry, ok := mgr.Get(ctx, sessionKeyTokenExpiry).(time.Time)
	if !ok {
		return time.Time{}, errors.New("no token expiry value in session")
	}
	return expiry, nil
}

func (a *Authenticator) GetReturnTo(ctx context.Context, mgr SessionManager) (string, error) {
	returnTo, ok := mgr.Get(ctx, sessionKeyReturnTo).(string)
	if !ok {
		return "", errors.New("no return to value in session")
	}
	return returnTo, nil
}

// IsAuthenticated returns true if the session contains an access token.
func (a *Authenticator) IsAuthenticated(ctx context.Context, mgr SessionManager) bool {
	tkn, err := a.GetAccessToken(ctx, mgr)
	return tkn != "" && err == nil
}

// IsAccessTokenExpired returns true if the access token has expired.
func (a *Authenticator) IsAccessTokenExpired(ctx context.Context, mgr SessionManager) bool {
	expiry, err := a.GetTokenExpiry(ctx, mgr)
	if err != nil || expiry.IsZero() {
		return true
	}
	return time.Now().After(expiry)
}

// SaveTokens saves the access token and data in the session.
func (a *Authenticator) SaveTokens(ctx context.Context, mgr SessionManager, token *TokenResponse) {
	mgr.Put(ctx, sessionKeyAccessToken, token.AccessToken)
	mgr.Put(ctx, sessionKeyTokenExpiry, tokenExpiry(token.ExpiresIn))
	mgr.Put(ctx, sessionKeyRefreshToken, token.RefreshToken)
}

// ClearAuth removes all authentication-related keys from the session.
func (a *Authenticator) ClearAuth(ctx context.Context, mgr SessionManager) {
	mgr.Remove(ctx, sessionKeyAccessToken)
	mgr.Remove(ctx, sessionKeyRefreshToken)
	mgr.Remove(ctx, sessionKeyTokenExpiry)
	mgr.Remove(ctx, sessionKeyUserProfile)
}

// ClearState removes all data related to an authorization exchange from the session.
func (a *Authenticator) ClearState(ctx context.Context, mgr SessionManager) {
	mgr.Remove(ctx, sessionKeyState)
	mgr.Remove(ctx, sessionKeyCodeVerifier)
}

// tokenExpiry calculates the absolute expiry time from an ExpiresIn value.
// A 30-second buffer is applied to account for clock skew.
func tokenExpiry(expiresIn int64) time.Time {
	return time.Now().Add(time.Duration(expiresIn)*time.Second - 30*time.Second)
}
