// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import 'htmx.org'
import './htmx.js'
import 'htmx-ext-response-targets'
import 'htmx-ext-multi-swap'
import 'htmx-ext-ajax-header'
// hyperscript
import _hyperscript from 'hyperscript.org/dist/_hyperscript.js'
_hyperscript.browserInit()
import './timestamps.js'
// CSS anchor positioning polyfill for browsers that don't support it natively.
// https://caniuse.com/css-anchor-positioning
import '@oddbird/css-anchor-positioning'
