// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/session"
)

func LoginHandler(res http.ResponseWriter, req *http.Request, authenticator *Authenticator) {
	state, err := generateRandomState()
	if err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Cannot generate state.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	if err := session.StoreState(req.Context(), state); err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Cannot save state.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	http.Redirect(res, req, authenticator.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func CallbackHandler(res http.ResponseWriter, req *http.Request, authenticator *Authenticator, code, state string) {
	if req.URL.Path != "/login/auth0/callback" {
		slogctx.FromCtx(req.Context()).
			Error("Invalid request.")
		http.NotFound(res, req)
	}

	sessionState, err := session.GetState(req.Context())
	if err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Invalid state.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	if state != sessionState {
		slogctx.FromCtx(req.Context()).
			Error("Unauthorized. Invalid state.")
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	// Exchange an authorization code for a tokens.
	tokens, err := authenticator.Exchange(req.Context(), code)
	if err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Unauthorized. Invalid token.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	idToken, err := authenticator.VerifyIDToken(req.Context(), tokens)
	if err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Unauthorized. Invalid token.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	// Store the user profile in the session.
	err = session.StoreTokens(req.Context(), idToken, tokens.AccessToken)
	if err != nil {
		slogctx.FromCtx(req.Context()).
			Error("Unable to store tokens.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)

		return
	}
}

func LogoutHandler(res http.ResponseWriter, req *http.Request) {
	logoutURL, err := generateLogoutURL(req)
	if err != nil {
		slog.Error("Auth0LogoutHandler: invalid Auth0 domain.",
			slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	// Clear the user session from the local session storage.
	if err := session.ClearSession(req.Context()); err != nil {
		slogctx.FromCtx(req.Context()).Warn("Failed to clear user session.", slog.Any("error", err))
	}

	// Redirect the user to logout from Auth0.
	http.Redirect(res, req, logoutURL.String(), http.StatusTemporaryRedirect)
}

// generateRandomState generates a random string that can be used as a state
// value for a user session. The string is safe to encode directly in the URL.
func generateRandomState() (string, error) {
	randomBytes := make([]byte, 64)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("unable to generate random bytes for state: %w", err)
	}

	state := url.QueryEscape(string(randomBytes))

	return state, nil
}
