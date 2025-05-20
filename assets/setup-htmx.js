// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import 'htmx.org'
import 'idiomorph'
import 'htmx-ext-sse'
// Use response targets extension to display error messages in a dedicated div.
//
// https://htmx.org/extensions/response-targets/
import 'htmx-ext-response-targets'

import htmx from 'htmx.org/dist/htmx.esm'
window.htmx = htmx

// Don't do nested oob swaps, only process nested oob swaps when *adjacent* to main target.
htmx.config.allowNestedOobSwaps = false

// Allow event logging.
//
// https://htmx.org/docs/#logging
// htmx.logger = function (elt, event, data) {
//   if (console) {
//     console.log(event, elt, data)
//   }
// }

// Log all events.
//
// https://v1.htmx.org/docs/#debugging
// htmx.logAll();
