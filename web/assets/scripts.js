// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import 'htmx.org'
import './htmx.js'
import 'htmx-ext-response-targets'
import 'htmx-ext-sse'
import 'htmx-ext-ajax-header'
import 'htmx-ext-preload'
// hyperscript
import _hyperscript from 'hyperscript.org/dist/_hyperscript.js'
_hyperscript.browserInit()
// custom element for timestamps
import './timestamps.js'
// tailwindplus
// import '@tailwindplus/elements';
import './imgproxy.js'
