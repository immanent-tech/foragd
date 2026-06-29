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

// Listen for scroll events and update the progress bar on article views.
const content = document.getElementById('content')
window.addEventListener('scroll', () => {
  if (!window.location.pathname.startsWith('/view/article/')) return

  const max = content.scrollHeight - content.clientHeight
  const pct = max > 0 ? (content.scrollTop / max) * 100 : 0
  const pb = document.querySelector('.progress-bar')
  if (pb) pb.style.width = pct + '%'
})
document.body.addEventListener('htmx:afterSettle', () => {
  if (window.location.pathname.startsWith('/view/article/')) {
    const pb = document.querySelector('.progress-bar')
    if (pb) pb.style.width = '0%'
  }
})

// import axe from 'axe-core'

// htmx.onLoad(function (content) {
//   axe
//     .run()
//     .then((results) => {
//       if (results.violations.length) {
//         console.log(results.violations)
//         throw new Error('Accessibility issues found')
//       }
//     })
//     .catch((err) => {
//       console.error('Something bad happened:', err.message)
//     })
// })
