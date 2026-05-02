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
import "hyperscript.org";

// Tailwind Plus.
// import '@tailwindplus/elements'

// Relative time custom element.
import "@github/relative-time-element";

// Custom element for youtube player.
import "./embed-youtube";

// Masonry grid layout.

import { GridRowsMasonry } from "grid-rows-masonry";

function initMasonry() {
  const grid = document.getElementById("grid-objects");
  if (!grid) return;

  // Remove any stray text/comment nodes that confuse the ResizeObserver
  Array.from(grid.childNodes).forEach((node) => {
    if (node.nodeType !== Node.ELEMENT_NODE) node.remove();
  });

  // // Tear down existing instance FIRST to stop its MutationObserver
  // if (grid._masonryInstance) {
  //   grid._masonryInstance.destroy();
  //   grid._masonryInstance = null;
  // }

  // // Force layout recalc before Masonry checks computed style.
  // void grid.offsetHeight; // triggers reflow.

  // const computed = window.getComputedStyle(grid);
  // if (computed.display !== "grid") {
  //   requestAnimationFrame(initMasonry);
  //   return;
  // }

  // Create instance.
  grid._masonryInstance = new GridRowsMasonry(grid);
}

htmx.onLoad(function (target) {
  initMasonry();
});

// htmx.on("htmx:beforeSwap", function (event) {
//   if (event.detail.target?.id === "grid-objects") {
//     const grid = document.getElementById("grid-objects");
//     if (grid?._masonryInstance) {
//       grid._masonryInstance.destroy();
//       grid._masonryInstance = null;
//     }
//   }
// });

// After every HTMX settle into #grid-objects (infinite scroll, filter changes)
htmx.on("htmx:afterSettle", function (event) {
  // Remove any stray text/comment nodes that confuse the ResizeObserver
  const grid = document.getElementById("grid-objects");
  if (!grid) return;
  Array.from(grid.childNodes).forEach((node) => {
    if (node.nodeType !== Node.ELEMENT_NODE) node.remove();
  });

  //   if (event.detail.target?.id === "grid-objects") {
  //     initMasonry();
  //   }
});

// htmx.on("htmx:beforeHistorySave", function () {
//   if (grid._masonryInstance) {
//     grid._masonryInstance.destroy();
//   }
//   initMasonry();
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
