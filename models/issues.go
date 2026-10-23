// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"

	"github.com/immanent-tech/go-base/validation"
)

func (i *ReportIssueRequest) Validate() error {
	if err := validation.Validate.Struct(i); err != nil {
		return err
	}
	return nil
}

func (i *ReportIssueRequest) Sanitise() error {
	var err error
	i.UserEmail = validation.SanitizeInput(i.UserEmail)
	i.PageUrl, err = validation.SanitizeURLString(i.PageUrl)
	if err != nil {
		return fmt.Errorf("sanitise page URL: %w", err)
	}
	if i.ScreenshotURL != nil {
		*i.ScreenshotURL, err = validation.SanitizeURLString(*i.ScreenshotURL)
		if err != nil {
			return fmt.Errorf("sanitise page URL: %w", err)
		}
	}
	if i.Details != nil {
		cleanDetails := validation.SanitizeHTML(*i.Details)
		i.Details = &cleanDetails
	}
	return nil
}
