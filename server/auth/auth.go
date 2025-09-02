// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package auth contains objects and methods for handling user authentication.
package auth

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/auth0"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/server/session"
)

func init() {
	gob.Register(UserAuth{})
	gob.Register(sessions.Session{})
}

// Ensure the session manager implements the Gorilla Store interface.
//
// https://github.com/gorilla/sessions/blob/main/store.go
var _ sessions.Store = (*Authenticator)(nil)

var (
	// ErrInvalidData indicates that the authentication data is invalid.
	ErrInvalidData = errors.New("invalid authentication data")
	// ErrAuth indicates the authentication request failed. The wrapped error will contain specific details.
	ErrAuth = errors.New("authenticator")
)

// UserAuth contains provider independent details about user authentication.
type UserAuth struct {
	goth.User
}

// GetUserID retrieves a user ID from the user authentication.
func (u *UserAuth) GetUserID() string {
	id, _ := strings.CutPrefix(u.UserID, "auth0|")
	return id
}

// GetNickname retrieves a user nickname from the user authentication.
func (u *UserAuth) GetNickname() string {
	return u.NickName
}

// GetEmail retrieves a user email from the user authentication.
func (u *UserAuth) GetEmail() string {
	return u.Email
}

// Authenticator manages user authentication to a provider.
type Authenticator struct{}

// NewAuthenticator sets up a new authenticator instance for authenticating user sessions.
func NewAuthenticator(ctx context.Context) (*Authenticator, error) {
	err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("authenticator: %w", err)
	}
	authenticator := &Authenticator{}
	goth.UseProviders(
		auth0.New(auth0Config.ClientID, auth0Config.ClientSecret, auth0Config.domainURL(), auth0Config.Domain),
	)
	gothic.Store = authenticator
	gothic.GetProviderName = authenticator.GetProviderName
	// gothic.CompleteUserAuth = authenticator.CompleteUserAuth
	return authenticator, nil
}

// Get returns a cached user session from the store.
func (a *Authenticator) Get(req *http.Request, name string) (*sessions.Session, error) {
	slogctx.FromCtx(req.Context()).Debug("Fetching session.", slog.String("name", name))
	currentSession, ok := session.Manager.Get(req.Context(), name).(*sessions.Session)
	if !ok {
		var err error
		currentSession, err = a.New(req, name)
		if err != nil {
			return nil, fmt.Errorf("authenticator: unable to retrieve user session: %w", err)
		}
	}
	slogctx.FromCtx(req.Context()).Debug("Found existing session.", slog.String("name", name))
	return currentSession, nil
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
func (a *Authenticator) New(req *http.Request, name string) (*sessions.Session, error) {
	if !session.Manager.Exists(req.Context(), name) {
		newSession := &sessions.Session{
			Values:  make(map[any]any),
			Options: new(sessions.Options),
			ID:      models.NewID(models.SessionPFX),
		}
		session.Manager.Put(req.Context(), name, newSession)
		slogctx.FromCtx(req.Context()).Debug("Created new session.")
		return newSession, nil
	} else {
		return a.Get(req, name)
	}
}

// Save should persist session to the underlying store implementation.
func (a *Authenticator) Save(req *http.Request, _ http.ResponseWriter, s *sessions.Session) error {
	session.Manager.Put(req.Context(), s.Name(), s)
	slogctx.FromCtx(req.Context()).Debug("Saved session.")
	return nil
}

// CompleteUserAuth handles processing a callback from a login provider to verify the login request.
func (a *Authenticator) CompleteUserAuth(res http.ResponseWriter, req *http.Request) error {
	providerName, err := gothic.GetProviderName(req)
	if err != nil {
		return fmt.Errorf("authenticator: unable to determine provider: %w", err)
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("authenticator: unable to retrieve provider %s: %w", providerName, err)
	}

	value := session.Manager.GetString(req.Context(), providerName)
	if value == "" {
		return fmt.Errorf("%w: no session found", ErrAuth)
	}

	// defer a.Logout().ServeHTTP(res, req)
	sess, err := provider.UnmarshalSession(value)
	if err != nil {
		return fmt.Errorf("authenticator: unable to unmarshal session: %w", err)
	}

	err = validateState(req, sess)
	if err != nil {
		return fmt.Errorf("authenticator: validate session state failed: %w", err)
	}

	user, err := provider.FetchUser(sess)
	if err == nil {
		// user can be found with existing session data
		a.StoreUserAuth(req.Context(), UserAuth{User: user})
		return nil
	}

	params := req.URL.Query()
	if params.Encode() == "" && req.Method == http.MethodPost {
		err := req.ParseForm()
		if err != nil {
			return fmt.Errorf("authenticator: unable to parse parameters: %w", err)
		}
		params = req.Form
	}

	// get new token and retry fetch
	_, err = sess.Authorize(provider, params)
	if err != nil {
		return fmt.Errorf("authenticator: authorize session failed: %w", err)
	}

	session.Manager.Put(req.Context(), providerName, sess.Marshal())

	gu, err := provider.FetchUser(sess)
	if err != nil {
		return fmt.Errorf("authenticator: fetch user details failed: %w", err)
	}
	a.StoreUserAuth(req.Context(), UserAuth{User: gu})
	return nil
}

// GetAuthURL starts the authentication process with the requested provided.
// It will return a URL that should be used to send users to.
func (a *Authenticator) GetAuthURL(req *http.Request) (string, error) {
	// Get provider name.
	providerName, err := gothic.GetProviderName(req)
	if err != nil {
		return "", fmt.Errorf("authenticator: unable to determine provider: %w", err)
	}
	// Get provider.
	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return "", fmt.Errorf("authenticator: unable to retrieve provider %s: %w", providerName, err)
	}
	// Setup authentication through provider.
	sess, err := provider.BeginAuth(gothic.SetState(req))
	if err != nil {
		return "", fmt.Errorf("authenticator: could not initiate authentication with provider: %w", err)
	}
	// Get the provider authentication URL.
	url, err := sess.GetAuthURL()
	if err != nil {
		return "", fmt.Errorf("authenticator: could not retrieve auth URL: %w", err)
	}
	slogctx.FromCtx(req.Context()).Debug("Initialised provider", slog.String("provider", providerName))
	session.Manager.Put(req.Context(), providerName, sess.Marshal())
	return url, nil
}

// SetProviderName sets the provider to use for authentication.
func (a *Authenticator) SetProviderName(ctx context.Context, provider string) {
	session.Manager.Put(ctx, "provider", provider)
}

// GetProviderName gets the provider to use for authentication.
func (a *Authenticator) GetProviderName(req *http.Request) (string, error) {
	provider := session.Manager.GetString(req.Context(), "provider")
	if provider == "" {
		return "", fmt.Errorf("%w: no auth provider found", ErrAuth)
	}
	return provider, nil
}

// StoreUserAuth stores the user authentication session returned from a provider in the session store.
func (a *Authenticator) StoreUserAuth(ctx context.Context, user UserAuth) {
	session.Manager.Put(ctx, "user", user)
	slogctx.FromCtx(ctx).Debug("Stored user auth.")
}

// GetUserID retrieves the user ID of the currently authenticated user.
func (a *Authenticator) GetUserID(ctx context.Context) models.UserID {
	auth, found := a.GetUserAuth(ctx)
	if !found {
		return ""
	}
	return auth.GetUserID()
}

// GetUserAuth retrieves the user authentication session store.
func (a *Authenticator) GetUserAuth(ctx context.Context) (UserAuth, bool) {
	user, found := session.Manager.Get(ctx, "user").(UserAuth)
	if !found {
		return UserAuth{}, false
	}
	return user, true
}

// validateState ensures that the state token param from the original
// AuthURL matches the one included in the current (callback) request.
func validateState(req *http.Request, sess goth.Session) error {
	rawAuthURL, err := sess.GetAuthURL()
	if err != nil {
		return fmt.Errorf("validate state: %w", err)
	}

	authURL, err := url.Parse(rawAuthURL)
	if err != nil {
		return fmt.Errorf("validate state: %w", err)
	}

	reqState := gothic.GetState(req)

	originalState := authURL.Query().Get("state")
	if originalState != "" && (originalState != reqState) {
		return fmt.Errorf("%w: state token mismatch", ErrAuth)
	}
	return nil
}
