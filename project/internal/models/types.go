// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"
)

func (s *UserSignup) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(s)
}

func (s *SearchRequest) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(s)
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

	t.Claims = claims

	return nil
}
