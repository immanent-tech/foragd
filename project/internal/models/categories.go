// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"html"
)

// CategoryCount holds a document count for a Category.
type CategoryCount struct {
	Name  Category
	Count int64
}

// CleanCategories takes an array of Categories and "cleans" them, removing any
// HTML escape strings for better display.
func CleanCategory(category Category) Category {
	return html.UnescapeString(safePrinter.Sanitize(category))
}
