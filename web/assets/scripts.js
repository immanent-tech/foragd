// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import 'htmx-ext-ajax-header'
import 'htmx-ext-preload'
import 'htmx-ext-response-targets'
import 'htmx-ext-sse'
import 'htmx.org'
import './htmx.js'
// hyperscript
import _hyperscript from 'hyperscript.org/dist/_hyperscript.js'
_hyperscript.browserInit()
// custom element for timestamps
import './timestamps.js'
// tailwindplus
// import '@tailwindplus/elements';
import './debug.js'
import './imgproxy.js'

// Global shortcuts handling.
import Shortcut from './Shortcut.js'
/* Create a basic shortcut handler */
const myShortcutHandler = new Shortcut()

myShortcutHandler.register('Ctrl+K', () => {
  htmx.trigger(document.body, 'customCtrlK')
})
