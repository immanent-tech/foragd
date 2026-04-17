// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/foragd/config"
)

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	return resty.New().SetHeader("User-Agent", config.AppName+"/"+config.Version)
})

func init() {
	gob.Register(UserProfile{})
	gob.Register(time.Time{})
	gob.Register(map[string]string{})
}

// generateCodeVerifier creates a cryptographically random PKCE code verifier.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generating code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge derives the S256 PKCE code challenge from a verifier.
func codeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateState creates a cryptographically random OAuth2 state parameter.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
