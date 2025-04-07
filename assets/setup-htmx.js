// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

import "idiomorph";
import "htmx-ext-sse";

import htmx from "htmx.org/dist/htmx.esm";
window.htmx = htmx;

// Allow event logging.
//
// https://htmx.org/docs/#logging
htmx.logger = function(elt, event, data) {
  if(console) {
      console.log(event, elt, data);
  }
}

// Log all events.
//
// https://v1.htmx.org/docs/#debugging
htmx.logAll();
