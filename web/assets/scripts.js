// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import htmx from 'htmx.org/dist/htmx.esm'
window.htmx = htmx
// htmx.logAll()

// import 'htmx-ext-head-support'
import 'htmx-ext-preload'
// import 'htmx-ext-response-targets'
import 'htmx-ext-sse'

// hyperscript
import _hyperscript from 'hyperscript.org/dist/_hyperscript.js'
_hyperscript.browserInit()

// custom element for timestamps
import './timestamps'

import 'transition-style'
