// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import 'htmx.org'
import './htmx-setup'

// hyperscript
import _hyperscript from 'hyperscript.org/dist/_hyperscript.js'
_hyperscript.browserInit()

// custom element for timestamps
import './timestamps'

htmx.logger = function (elt, event, data) {
  if (console) {
    console.log(event, elt, data)
  }
}

// Log all events.
//
// https://v1.htmx.org/docs/#debugging
htmx.logAll()
