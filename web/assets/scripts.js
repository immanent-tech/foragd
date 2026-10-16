/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

import './trusted-types'

import './htmx'
import './tailwindplus'
// Relative time custom element.
import '@github/relative-time-element'
// Custom element for YouTube player.
import './embed-youtube'
// Android billing.
import 'leaflet-html'
import 'leaflet/dist/leaflet.css'
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

// For keyboard navigation in search suggestions popover.
document.addEventListener('keydown', (e) => {
  const input = document.getElementById('global-search')
  const popover = document.getElementById('search-suggestions-popover')
  if (!input || !popover) return
  if (e.target !== input && !popover.contains(e.target)) return

  const items = [...popover.querySelectorAll('[role="option"]')]
  const idx = items.indexOf(document.activeElement)
  const inInput = e.target === input

  if (e.key === 'ArrowDown' && items.length) {
    e.preventDefault()
    items[idx < items.length - 1 ? idx + 1 : 0].focus()
  } else if (e.key === 'ArrowUp' && items.length) {
    e.preventDefault()
    if (!inInput && idx === 0) input.focus()
    else items[idx > 0 ? idx - 1 : items.length - 1].focus()
  } else if (e.key === 'Escape') {
    popover.hidePopover?.()
    input.focus()
  } else if (e.key === 'Enter' && !inInput) {
    e.preventDefault()
    e.stopPropagation()
    document.activeElement.click()
  }
})

// // Or specific event debugging
// document.body.addEventListener('htmx:after:swap', function (e) {
//   console.log('Request config:', e.detail)
// })

// // Visual event indicators
// document.body.addEventListener('htmx:before:request', function (e) {
//   e.target.style.outline = '2px solid blue'
// })

// document.body.addEventListener('htmx:after:swap', function (e) {
//   e.target.style.outline = '2px solid green'
//   setTimeout(() => (e.target.style.outline = ''), 1000)
// })
