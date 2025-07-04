// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Allow event logging.

// htmx.logger = function (elt, event, data) {
//   if (console) {
//     console.log(event, elt, data)
//   }
// }

// Log all events.
//
// https://v1.htmx.org/docs/#debugging
// htmx.logAll()

// Action: history restore events.
window.addEventListener('htmx:historyRestore', (event) => {
  console.log('restored history.')
})
