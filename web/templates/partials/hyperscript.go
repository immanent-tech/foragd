// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

// HyperscriptCloseDropdown will close a parent dropdown element. Useful as an onclick action for dropdown menu items.
func HyperscriptCloseDropdown() string {
	return "remove @open from the closest parent .dropdown"
}

// HyperscriptOpenModal will open the modal with the given ID after the request has loaded. Useful as an action on menu items
// that launch modals.
func HyperscriptOpenModal(id string) string {
	return "on htmx:afterOnLoad wait 10ms then add @open to " + id
}

// HyperscriptCloseModalOnClick will close the modal with the given ID on click.
func HyperscriptCloseModalOnClick(id string) string {
	return "on click remove @open from " + id
}
