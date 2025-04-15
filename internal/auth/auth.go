// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/auth0"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/session"
)

// Ensure the session manager implements the Gorilla Store interface.
//
// https://github.com/gorilla/sessions/blob/main/store.go
var _ sessions.Store = (*Authenticator)(nil)

var (
	ErrStartAuthenticator = errors.New("could not create new authentication provider")
	ErrInvalidData        = errors.New("invalid authentication data")
)

type Authenticator struct {
	sessionMgr *session.Manager
}

// Get returns a cached user session from the store.
func (a *Authenticator) Get(req *http.Request, name string) (*sessions.Session, error) {
	currentSession, ok := a.sessionMgr.Get(req.Context(), name).(*sessions.Session)
	if !ok {
		return &sessions.Session{}, ErrInvalidData
	}
	slogctx.FromCtx(req.Context()).Debug("Found existing session.", slog.String("name", name))
	return currentSession, nil
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
func (a *Authenticator) New(req *http.Request, name string) (*sessions.Session, error) {
	if !a.sessionMgr.Exists(req.Context(), name) {
		newSession := &sessions.Session{
			Values:  make(map[any]any),
			Options: new(sessions.Options),
			ID:      id.NewID(id.Session),
		}
		a.sessionMgr.Put(req.Context(), name, newSession)
		slogctx.FromCtx(req.Context()).Debug("Created new session.")
		return newSession, nil
	} else {
		return a.Get(req, name)
	}
}

// Save should persist session to the underlying store implementation.
func (a *Authenticator) Save(req *http.Request, _ http.ResponseWriter, s *sessions.Session) error {
	a.sessionMgr.Put(req.Context(), s.Name(), s)
	slogctx.FromCtx(req.Context()).Debug("Saved session.")
	return nil
}

func NewAuthenticator(sessionMgr *session.Manager) (*Authenticator, error) {
	if err := loadConfigOnce(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStartAuthenticator, err)
	}

	authenticator := &Authenticator{
		sessionMgr: sessionMgr,
	}

	goth.UseProviders(
		auth0.New(auth0Config.ClientID, auth0Config.ClientSecret, auth0Config.DomainURL(), auth0Config.Domain),
	)
	gothic.Store = authenticator
	gothic.GetProviderName = GetAuthProvider
	gothic.CompleteUserAuth = authenticator.CompleteUserAuth

	return authenticator, nil
}

func (a *Authenticator) CompleteUserAuth(res http.ResponseWriter, req *http.Request) (goth.User, error) {
	providerName, err := gothic.GetProviderName(req)
	if err != nil {
		return goth.User{}, err
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return goth.User{}, err
	}

	value := a.sessionMgr.GetString(req.Context(), providerName)
	// value, err := gothic.GetFromSession(providerName, req)
	if value == "" {
		return goth.User{}, errors.New("no auth provider session")
	}

	defer handlers.Logout().ServeHTTP(res, req)
	sess, err := provider.UnmarshalSession(value)
	if err != nil {
		return goth.User{}, err
	}

	err = validateState(req, sess)
	if err != nil {
		return goth.User{}, err
	}

	user, err := provider.FetchUser(sess)
	if err == nil {
		// user can be found with existing session data
		return user, err
	}

	params := req.URL.Query()
	if params.Encode() == "" && req.Method == "POST" {
		req.ParseForm()
		params = req.Form
	}

	// get new token and retry fetch
	_, err = sess.Authorize(provider, params)
	if err != nil {
		return goth.User{}, err
	}

	a.sessionMgr.Put(req.Context(), providerName, sess.Marshal())

	// err = StoreInSession(providerName, sess.Marshal(), req, res)
	// if err != nil {
	// 	return goth.User{}, err
	// }

	gu, err := provider.FetchUser(sess)
	spew.Dump(gu)
	return gu, err

	// return goth.User{}, nil
}

func (a *Authenticator) GetAuthURL(res http.ResponseWriter, req *http.Request) (string, error) {
	providerName, err := gothic.GetProviderName(req)
	if err != nil {
		return "", err
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return "", err
	}
	sess, err := provider.BeginAuth(gothic.SetState(req))
	if err != nil {
		return "", err
	}

	url, err := sess.GetAuthURL()
	if err != nil {
		return "", err
	}

	a.sessionMgr.Put(req.Context(), providerName, sess.Marshal())

	return url, err
}

// validateState ensures that the state token param from the original
// AuthURL matches the one included in the current (callback) request.
func validateState(req *http.Request, sess goth.Session) error {
	rawAuthURL, err := sess.GetAuthURL()
	if err != nil {
		return err
	}

	authURL, err := url.Parse(rawAuthURL)
	if err != nil {
		return err
	}

	reqState := gothic.GetState(req)

	originalState := authURL.Query().Get("state")
	if originalState != "" && (originalState != reqState) {
		return errors.New("state token mismatch")
	}
	return nil
}
