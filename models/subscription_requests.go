// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"github.com/immanent-tech/foragd/validation"
)

func (r *GroupSubscriptionSuggestionRequest) Valid() error {
	return validation.Validate.Struct(r)
}

func (r *GroupSubscriptionSuggestionRequest) Sanitise() error {
	r.Text = validation.SanitizeString(r.Text)
	return nil
}
