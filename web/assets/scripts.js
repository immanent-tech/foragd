// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import htmx from "htmx.org/dist/htmx.esm";
window.htmx = htmx;

// import 'htmx-ext-head-support'
import "htmx-ext-preload";
// import 'htmx-ext-response-targets'
import "htmx-ext-sse";
// import 'idiomorph/htmx'

// hyperscript
import _hyperscript from "hyperscript.org/dist/_hyperscript.js";
_hyperscript.browserInit();

// Tailwind Plus.
// import '@tailwindplus/elements'

// Relative time custom element.
import "@github/relative-time-element";

// custom element for youtube player.
import "./embed-youtube";

import { GridRowsMasonry } from "grid-rows-masonry";

function initMasonry() {
  const grid = document.getElementById("grid-objects");
  if (!grid) return;

  // Force layout recalc before Masonry checks computed style.
  void grid.offsetHeight; // triggers reflow.

  const computed = window.getComputedStyle(grid);
  if (computed.display !== "grid") {
    requestAnimationFrame(updateLayout);
    return;
  }

  // Store instance on the element so we can destroy/recreate on HTMX swaps.
  if (grid._masonryInstance) {
    grid._masonryInstance.destroy();
  }

  // Create instance.
  grid._masonryInstance = new GridRowsMasonry(grid);
}

htmx.onLoad(function (target) {
  initMasonry();
});

// After every HTMX swap into #grid-objects (infinite scroll, filter changes)
htmx.on("htmx:afterSwap", function (event) {
  if (event.detail.target?.id === "grid-objects") initMasonry();
});

// htmx.on("htmx:beforeHistorySave", function () {
//   if (grid._masonryInstance) {
//     grid._masonryInstance.destroy();
//   }
// });

// // After every HTMX swap into #grid-objects (infinite scroll, filter changes)
// document.addEventListener("htmx:afterSwap", (e) => {
//   if (e.detail.target?.id === "grid-objects") initMasonry();
// });

// Make sure back button after logout does not show cached data.
//
// https://web.dev/articles/bfcache?utm_source=devtools#update-data-after-restore
window.addEventListener("pageshow", (event) => {
  if (event.persisted && !document.cookie.match("foragd-session")) {
    // Force a reload if the user has logged out.
    location.reload();
  }
});
