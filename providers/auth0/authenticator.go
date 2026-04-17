// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/auth0/go-auth0/v2/authentication"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/session"
)

// Session key constants used to store values in the SCS session.
const (
	sessionKeyAccessToken  = "access_token"
	sessionKeyRefreshToken = "refresh_token"
	sessionKeyIDToken      = "id_token"
	sessionKeyTokenExpiry  = "token_expiry"
	sessionKeyUserProfile  = "user_profile"
	sessionKeyState        = "oauth_state"
	sessionKeyCodeVerifier = "pkce_code_verifier"
	sessionKeyReturnTo     = "return_to"
)

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

var authClient Authenticator

// initAuthenticator will the setup and initialisation of the Auth0 tenant. It can be called multiple times but will only
// perform initialisation once (so it can be lazily loaded by calling it before any Auth0 actions).
var initAuthenticator = func(ctx context.Context) error {
	err := sync.OnceValue(func() error {
		err := loadConfigOnce()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		provider, err := oidc.NewProvider(
			ctx,
			"https://"+cfg.Domain+"/",
		)
		if err != nil {
			return fmt.Errorf("create provider: %w", err)
		}

		conf := oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email"},
		}
		authClient = Authenticator{
			Provider: provider,
			Config:   conf,
		}
		return nil
	})()
	if err != nil {
		return err
	}
	return nil
}

// postToken sends a POST request to the Auth0 token endpoint and decodes the response.
func (a *Authenticator) postToken(ctx context.Context, form url.Values) (*TokenResponse, error) {
	client := loadHTTPClient()

	var token TokenResponse
	var errResult authentication.Error
	resp, err := client.R().
		SetContext(ctx).
		SetBody(form).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetResult(&token).
		SetError(&errResult).
		Post(a.Config.Endpoint.TokenURL)
	switch {
	case resp.IsError():
		return nil, fmt.Errorf("post token: %d: %s", resp.StatusCode(), resp.Status())
	case err != nil:
		return nil, fmt.Errorf("post token: %w", err)
	}

	return &token, nil
}

// Exchange handles verifying and exchanging the authorization code for an access token. It also extracts the ID token
// and user profile.
func Exchange(ctx context.Context, code, verifier string) (*TokenResponse, *UserProfile, error) {
	if err := initAuthenticator(ctx); err != nil {
		return nil, nil, fmt.Errorf("init authenticator: %w", err)
	}
	// token, err := AuthClient.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	token, err := authClient.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code for token: %w", err)
	}

	// Verify token.
	idToken, idTokenHash, err := VerifyIDToken(ctx, token)
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
func RefreshTokens(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if err := initAuthenticator(ctx); err != nil {
		return nil, fmt.Errorf("init authenticator: %w", err)
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", authClient.Config.ClientID)
	form.Set("client_secret", authClient.Config.ClientSecret)
	form.Set("refresh_token", refreshToken)

	return authClient.postToken(ctx, form)
}

// VerifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, string, error) {
	if err := initAuthenticator(ctx); err != nil {
		return nil, "", fmt.Errorf("init authenticator: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, "", ErrNoIDToken
	}
	oidcConfig := &oidc.Config{
		ClientID: authClient.ClientID,
	}
	id, err := authClient.Verifier(oidcConfig).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", fmt.Errorf("unable to verify token: %w", err)
	}
	return id, rawIDToken, nil
}

// AuthURLResult holds the generated authorization URL along with the state
// and PKCE code verifier that must be stored in the session before redirecting.
type AuthURLResult struct {
	URL          string
	State        string
	CodeVerifier string
}

// GenerateAuthURL constructs the Auth0 Universal Login redirect URL using PKCE.
func GenerateAuthURL(req *http.Request) (*AuthURLResult, error) {
	if err := initAuthenticator(req.Context()); err != nil {
		return nil, fmt.Errorf("init authenticator: %w", err)
	}

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
		planID := req.URL.Query().Get(models.ParamPlanID)
		session.Save(req.Context(), models.ParamPlanID, planID)
		authURL = authClient.AuthCodeURL(state,
			oauth2.SetAuthURLParam("screen_hint", "signup"),
			// oauth2.S256ChallengeOption(codeChallenge(verifier)),
		)
	case "/login":
		// authURL = AuthClient.AuthCodeURL(state, oauth2.S256ChallengeOption(codeChallenge(verifier)))
		authURL = authClient.AuthCodeURL(state)
	}

	return &AuthURLResult{
		URL:          authURL,
		State:        state,
		CodeVerifier: verifier,
	}, nil
}

// GenerateLogoutURL generates URL to log the user out from the auth backend.
func GenerateLogoutURL(req *http.Request) (*url.URL, error) {
	if err := initAuthenticator(req.Context()); err != nil {
		return nil, fmt.Errorf("init authenticator: %w", err)
	}
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

func PutState(req *http.Request, state string) {
	session.Save(req.Context(), sessionKeyState, state)
}

func PutCodeVerifier(req *http.Request, verifier string) {
	session.Save(req.Context(), sessionKeyCodeVerifier, verifier)
}

func PutReturnTo(req *http.Request, path string) {
	session.Save(req.Context(), sessionKeyReturnTo, path)
}

func GetState(req *http.Request) (string, error) {
	state, err := session.Restore[string](req.Context(), sessionKeyState)
	if err != nil {
		return "", fmt.Errorf("get state: %w", err)
	}
	return state, nil
}

func GetCodeVerifier(req *http.Request) (string, error) {
	verifier, err := session.Restore[string](req.Context(), sessionKeyCodeVerifier)
	if err != nil {
		return "", fmt.Errorf("get verifier: %w", err)
	}
	return verifier, nil
}

func GetAccessToken(req *http.Request) (string, error) {
	tkn, err := session.Restore[string](req.Context(), sessionKeyAccessToken)
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	return tkn, nil
}

func GetRefreshToken(req *http.Request) (string, error) {
	tkn, err := session.Restore[string](req.Context(), sessionKeyRefreshToken)
	if err != nil {
		return "", fmt.Errorf("get refresh token: %w", err)
	}
	return tkn, nil
}

func GetIDToken(req *http.Request) (*oidc.IDToken, error) {
	tkn, err := session.Restore[oidc.IDToken](req.Context(), sessionKeyIDToken)
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &tkn, nil
}

func GetTokenExpiry(req *http.Request) (time.Time, error) {
	expiry, err := session.Restore[time.Time](req.Context(), sessionKeyTokenExpiry)
	if err != nil {
		return time.Time{}, fmt.Errorf("get token expiry: %w", err)
	}
	return expiry, nil
}

func GetReturnTo(req *http.Request) (string, error) {
	returnTo, err := session.Restore[string](req.Context(), sessionKeyReturnTo)
	if err != nil {
		return "", fmt.Errorf("get return to: %w", err)
	}
	return returnTo, nil
}

// IsAuthenticated returns true if the session contains an access token.
func IsAuthenticated(req *http.Request) bool {
	tkn, err := GetAccessToken(req)
	return tkn != "" && err == nil
}

// IsAccessTokenExpired returns true if the access token has expired.
func IsAccessTokenExpired(req *http.Request) bool {
	expiry, err := GetTokenExpiry(req)
	if err != nil || expiry.IsZero() {
		return true
	}
	return time.Now().After(expiry)
}

// SaveTokens saves the access token and data in the session.
func SaveTokens(ctx context.Context, token *TokenResponse) {
	session.Save(ctx, sessionKeyAccessToken, token.AccessToken)
	session.Save(ctx, sessionKeyIDToken, token.IDToken)
	session.Save(ctx, sessionKeyTokenExpiry, tokenExpiry(token.ExpiresIn))
	if token.RefreshToken != "" {
		session.Save(ctx, sessionKeyRefreshToken, token.RefreshToken)
	}
}

// ClearAuth removes all authentication-related keys from the session.
func ClearAuth(req *http.Request) {
	session.Remove(req.Context(), sessionKeyAccessToken)
	session.Remove(req.Context(), sessionKeyRefreshToken)
	session.Remove(req.Context(), sessionKeyIDToken)
	session.Remove(req.Context(), sessionKeyTokenExpiry)
	session.Remove(req.Context(), sessionKeyUserProfile)
}

// ClearState removes all data related to an authorization exchange from the session.
func ClearState(req *http.Request) {
	session.Remove(req.Context(), sessionKeyState)
	session.Remove(req.Context(), sessionKeyCodeVerifier)
}

// tokenExpiry calculates the absolute expiry time from an ExpiresIn value.
// A 30-second buffer is applied to account for clock skew.
func tokenExpiry(expiresIn int64) time.Time {
	return time.Now().Add(time.Duration(expiresIn)*time.Second - 30*time.Second)
}
