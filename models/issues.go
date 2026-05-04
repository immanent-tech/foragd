// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "github.com/immanent-tech/foragd/validation"

func (i *ReportIssueRequest) Valid() error {
	if err := validation.Validate.Struct(i); err != nil {
		return err
	}
	return nil
}

func (i *ReportIssueRequest) Sanitise() error {
	if i.Details != nil {
		cleanDetails := validation.SanitizeString(*i.Details)
		i.Details = &cleanDetails
	}
	return nil
}
