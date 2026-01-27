// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

func (m *DocMetadata) GetTimestamp() string {
	switch {
	case m.UpdatedAt != "":
		return m.UpdatedAt
	case m.CreatedAt != "":
		return m.CreatedAt
	default:
		return "Unknown"
	}
}
