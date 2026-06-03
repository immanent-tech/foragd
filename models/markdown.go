// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

func (fm *MarkdownFrontMatter) GetCreatedDate() string {
	return fm.CreatedAt
}

func (fm *MarkdownFrontMatter) GetUpdatedDate() string {
	if fm.UpdatedAt != nil {
		return *fm.UpdatedAt
	}
	return ""
}
