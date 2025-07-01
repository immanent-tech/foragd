// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import "github.com/joshuar/go-templ-daisyui/attributes"

var (
	// ContentID points to the element containing the main content of the page.
	ContentID = attributes.ID("content")
	// ErrorID points to an element that can be used to display error messages to the user.
	ErrorID = attributes.ID("error")
	// ModalContainerID points to an element that holds a modal.
	ModalContainerID = attributes.ID("modals")
	// ModalID points to an element that can be used to render a modal.
	ModalID = attributes.ID("modal")
	// NotificationsID points to an element that can be used for displaying notifications to the user.
	NotificationsID = attributes.ID("notifications")
)
