// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

func Auth0Login(res http.ResponseWriter, req *http.Request, authenticator *auth0.Authenticator) {
	state, err := generateRandomState()
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Cannot generate state.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	if err := session.StoreState(req.Context(), state); err != nil {
		logging.FromContext(req.Context()).
			Error("Cannot save state.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	http.Redirect(res, req, authenticator.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func Auth0Callback(res http.ResponseWriter, req *http.Request, authenticator *auth0.Authenticator, code, state string) {
	if req.URL.Path != "/login/auth0/callback" {
		logging.FromContext(req.Context()).
			Error("Invalid request.")
		http.NotFound(res, req)
	}

	sessionState, err := session.GetState(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Invalid state.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)

		return
	}

	if state != sessionState {
		logging.FromContext(req.Context()).
			Error("Unauthorized. Invalid state.")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Exchange an authorization code for a tokens.
	tokens, err := authenticator.Exchange(req.Context(), code)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Unauthorized. Invalid token.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	idToken, err := authenticator.VerifyIDToken(req.Context(), tokens)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Unauthorized. Invalid token.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Store the user profile in the session.
	err = session.StoreTokens(req.Context(), idToken, tokens.AccessToken)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Unable to store tokens.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Redirect to logged in page.
	req.Header.Add("Content-Type", "")
	http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
}

func Auth0LogoutHandler(res http.ResponseWriter, req *http.Request, authenticator *auth0.Authenticator) {
	logoutURL, err := authenticator.LogoutURL(req)
	if err != nil {
		logging.LogReq(req, http.StatusInternalServerError).
			Error("Auth0LogoutHandler: invalid Auth0 domain.",
				slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

	// Clear the user session from the local session storage.
	if err := session.ClearSession(req.Context()); err != nil {
		logging.FromContext(req.Context()).Warn("Failed to clear user session.", slog.Any("error", err))
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
