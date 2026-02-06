// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

func (m *MarkdownFile) GetTimestamp() string {
	switch {
	case m.Frontmatter.UpdatedAt != "":
		return m.Frontmatter.UpdatedAt
	case m.Frontmatter.CreatedAt != "":
		return m.Frontmatter.CreatedAt
	default:
		return "Unknown"
	}
}
