// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "time"

func (fm *MarkdownFrontMatter) GetCreatedDate() time.Time {
	created, _ := time.Parse(time.DateOnly, fm.CreatedAt)
	return created
}

func (fm *MarkdownFrontMatter) GetUpdatedDate() time.Time {
	if fm.UpdatedAt != nil {
		updated, _ := time.Parse(time.DateOnly, *fm.UpdatedAt)
		return updated
	}
	return time.Time{}
}
