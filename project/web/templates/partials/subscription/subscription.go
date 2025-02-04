// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package subscription

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

func generateNameInput(req *models.APISubscriptionRequest) templ.Component {
	input := components.BuildTextInput(
		components.WithFormControl(),
		components.WithResponsiveSize[*components.TextInputProps](components.SM),
		components.WithInsideLabels(
			components.Text("Nickname", components.WithTextSize(components.TextSM)),
			components.HelperDropdown(
				"Optional. Replace the name of the feed with your own custom nickname.",
				components.WithOpenFrom[*components.DropdownProps](components.OpenLeft),
				components.WithOpenOnHover(),
			),
		),
		components.WithName[*components.TextInputProps]("name"),
		components.WithPlaceholder[*components.TextInputProps]("Cool feed"),
	)
	// Check for validation or input errors and mark the field with error status.
	if req.Name.IsSpecified() {
		name, err := req.Name.Get()
		if err != nil || req.ValidationErrors["Name"] != "" {
			input.SetColor(components.ColorStateError, true)

			if err != nil {
				input.SetBottomLeftLabel(err.Error())
			}

			if req.ValidationErrors["Name"] != "" {
				input.SetBottomRightLabel(req.ValidationErrors["Name"])
			}
		}

		input.SetValue(name)
	}

	return components.TextInput(components.FromTextInputProps(input))
}

// subscriptionURL generates the text input for the subscription URL.
func generateURLInput(req *models.APISubscriptionRequest) templ.Component {
	input := components.BuildTextInput(
		components.WithFormControl(),
		components.WithResponsiveSize[*components.TextInputProps](components.SM),
		components.WithInsideLabels(
			components.Text("URL", components.WithTextSize(components.TextSM)),
			components.HelperDropdown(
				"The URL for the feed.",
				components.WithOpenFrom[*components.DropdownProps](components.OpenLeft),
				components.WithOpenOnHover(),
			),
		),
		components.WithName[*components.TextInputProps]("url"),
		components.AsURL(),
		components.WithColor[*components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[*components.TextInputProps]("https://cool.feed.lol/rss"),
		components.WithValidationRequired[*components.TextInputProps](),
	)
	// Check for validation errors and mark the field with error status.
	if req.URL != "" {
		input.SetValue(req.URL)

		if req.ValidationErrors["URL"] != "" {
			input.SetColor(components.ColorStateError, true)
			input.SetBottomRightLabel(req.ValidationErrors["URL"])
		}
	}

	return components.TextInput(components.FromTextInputProps(input))
}

func generateCategoriesInput(id models.SubscriptionID) templ.Component {
	return components.Form(
		components.WithID[*components.FormProps]("add_category_form"),
		components.WithFormComponents(
			components.TextInput(
				components.WithName[*components.TextInputProps]("category"),
				components.WithFormControl(),
				components.WithResponsiveSize[*components.TextInputProps](components.SM),
				components.WithInsideLabels(
					components.Text("Category:", components.WithTextSize(components.TextSM)),
					components.Button(
						components.WithResponsiveSize[*components.ButtonProps](components.SM),
						components.WithButtonShape(components.ButtonCircle, false),
						components.WithButtonContent(components.AsIconContent("fa-plus")),
					),
				),
			),
		),
		components.WithAttributes[*components.FormProps](templ.Attributes{
			"hx-put":               "/subscription/" + id + "/category",
			"hx-target":            "#categories",
			"hx-swap":              "beforeend",
			"hx-on::after-request": "if(event.detail.successful) this.reset()",
		}),
	)
}

func generateCategoriesList(id models.SubscriptionID, allCategories ...models.Category) templ.Component {
	categories := make([]templ.Component, 0, len(allCategories))

	for _, category := range allCategories {
		categories = append(categories,
			generateCategoryItem(id, category),
		)
	}

	// Categories in an unordered list.
	return components.UnorderedList(
		components.WithID[*components.List]("categories"),
		components.WithItems[*components.List](categories...),
		components.WithAttributes[*components.List](templ.Attributes{
			"hx-target": "closest li",
			"hx-swap":   "outerHTML swap:1s",
		}),
	)
}

func generateCategoryItem(id models.SubscriptionID, category models.Category) templ.Component {
	return components.JoinHorizontally(
		// The category displayed as a badge.
		partials.CategoryBadge(category),
		// Hidden input that will submit the category with the
		// add subscription form.
		components.HiddenInput(
			components.WithName[*components.HiddenInputProps]("categories[]"),
			components.WithValue[*components.HiddenInputProps](category),
			components.WithAttributes[*components.HiddenInputProps](templ.Attributes{
				"form": "add_subscription_form",
			}),
		),
		// Button to remove category.
		components.Button(
			components.WithResponsiveSize[*components.ButtonProps](components.XS),
			components.WithButtonShape(components.ButtonCircle, false),
			components.WithButtonContent(components.AsIconContent("fa-minus")),
			components.WithAttributes[*components.ButtonProps](
				templ.Attributes{
					"hx-delete": "/subscription/" + id + "/category",
					// "hx-trigger": "change",
					// "hx-sync": "closest form:abort",
					// "form":    "update_categories_form",
				}),
		),
	)
}
