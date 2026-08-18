// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import htmx from 'htmx.org/dist/htmx.esm'
window.htmx = htmx

// hyperscript
import 'hyperscript.org'

// Relative time custom element.
import '@github/relative-time-element'

// Custom element for youtube player.
import './embed-youtube'

// Android billing.
import './android-billing'

// Make sure back button after logout does not show cached data.
//
// https://web.dev/articles/bfcache?utm_source=devtools#update-data-after-restore
window.addEventListener('pageshow', (event) => {
  if (event.persisted && !document.cookie.match('foragd-session')) {
    // Force a reload if the user has logged out.
    location.reload()
  }
})

// // Log all HTMX events
// htmx.logAll()

// // Or specific event debugging
// document.body.addEventListener('htmx:afterSettle', function (e) {
//   console.log('Request config:', e.detail)
// })

// // Visual event indicators
// document.body.addEventListener('htmx:beforeRequest', function (e) {
//   e.target.style.outline = '2px solid blue'
// })

// document.body.addEventListener('htmx:afterSwap', function (e) {
//   e.target.style.outline = '2px solid green'
//   setTimeout(() => (e.target.style.outline = ''), 1000)
// })
