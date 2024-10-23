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

package session

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc"
)

type Claims struct {
	Subject        string `json:"sub"`
	SessionID      string `json:"sid"`
	UserName       string `json:"name"`
	UserPictureURL string `json:"picture"`
	Issuer         string `json:"iss"`
	UserNickName   string `json:"nickname"`
	UpdatedAt      string `json:"updated_at"`
	Audience       string `json:"aud"`
	Expiry         uint64 `json:"exp"`
	IssuedAt       uint64 `json:"iat"`
}

type Tokens struct {
	IDToken     *oidc.IDToken
	Claims      *Claims
	AccessToken string
}

func (t *Tokens) UserID() string {
	return t.IDToken.Subject
}

func (t *Tokens) Nickname() string {
	return t.Claims.UserNickName
}

func (t *Tokens) Email() string {
	return t.Claims.UserName
}

func (t *Tokens) DecodeClaims() error {
	var claims Claims

	if err := t.IDToken.Claims(&claims); err != nil {
		return fmt.Errorf("cannot decode user claims from ID token: %w", err)
	}

	t.Claims = &claims

	return nil
}

func StoreTokens(ctx context.Context, idToken *oidc.IDToken, accessToken string) error {
	tokens := Tokens{
		IDToken:     idToken,
		AccessToken: accessToken,
	}

	if err := tokens.DecodeClaims(); err != nil {
		return fmt.Errorf("cannot store tokens: %w", err)
	}

	sessionManager.Put(ctx, profileSessionKey, tokens)

	return nil
}

func GetTokens(ctx context.Context) (*Tokens, error) {
	data := sessionManager.Get(ctx, profileSessionKey)
	tokens, ok := data.(Tokens)

	switch {
	case data == nil:
		return nil, ErrDataNotFound
	case ok:
		return &tokens, nil
	default:
		return nil, ErrInvalidData
	}
}

func UserID(ctx context.Context) (string, error) {
	tokens, err := GetTokens(ctx)
	if err != nil {
		return "", fmt.Errorf("could not retrieve user id: %w", err)
	}

	return tokens.IDToken.Subject, nil
}
