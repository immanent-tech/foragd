// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"html"

	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// CleanCategories takes an array of Categories and "cleans" them, removing any
// HTML escape strings for better display.
func CleanCategory(category Category) Category {
	return html.UnescapeString(safePrinter.Sanitize(category))
}

func GenerateCategoryItem(category Category) templ.Component {
	return components.JoinHorizontally(
		// The category displayed as a badge.
		partials.CategoryBadge(category),
		// Hidden input that will submit the category with the
		// add subscription form.
		components.HiddenInput(
			components.WithName[*components.HiddenInputProps]("categories[]"),
			components.WithValue[*components.HiddenInputProps](category),
			components.WithAttributes[*components.HiddenInputProps](templ.Attributes{
				"form": "new_subscription",
			}),
		),
		// Button to remove category.
		components.Button(
			components.WithResponsiveSize[*components.ButtonProps](components.XS),
			components.WithButtonShape(components.ButtonCircle, false),
			components.WithButtonContent(components.AsIconContent("fa-minus")),
			components.WithAttributes[*components.ButtonProps](
				templ.Attributes{
					"hx-delete": "/subscription/category",
				}),
		),
	)
}
