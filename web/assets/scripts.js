// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// htmx
import htmx from "htmx.org/dist/htmx.esm";
window.htmx = htmx;

// hyperscript
import "hyperscript.org";

// Relative time custom element.
import "@github/relative-time-element";

// Custom element for youtube player.
import "./embed-youtube";

// Make sure back button after logout does not show cached data.
//
// https://web.dev/articles/bfcache?utm_source=devtools#update-data-after-restore
window.addEventListener("pageshow", (event) => {
  if (event.persisted && !document.cookie.match("foragd-session")) {
    // Force a reload if the user has logged out.
    location.reload();
  }
});

// Listen for scroll events and update the progress bar.
if (window.location.pathname.startsWith("/view/article/")) {
  window.addEventListener("scroll", () => {
    const el = document.documentElement;
    const pct = (el.scrollTop / (el.scrollHeight - el.clientHeight)) * 100;
    const pb = document.querySelector(".progress-bar");
    if (pb) pb.style.width = pct + "%";
  });
}
