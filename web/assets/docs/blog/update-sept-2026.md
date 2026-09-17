+++
title = "Foragd September 2026 Update"
page_title = "Foragd RSS/Atom/JSON Feed Reader - September 2026 Update"
description = "A quick summary of updates to Foragd for September 2026"
created_at = "2026-09-18"
updated_at = "2026-09-18"
image = "/content/logo-vertical-light.webp"
author = "Joshua Rich"
slug = "update-sept-2026"
+++

A quick update post summarizing the updates in [Foragd](https://foragd.app) for September 2026.

**What’s new in this update:**

- **GeoRSS support:**. There is an experimental [Map](https://foragd-dev.astralcars.com/map) page that shows the latest
  50 articles that have geo data, on a world map
  ([screenshot](https://foragd.app/content/screenshots/screenshot-feature-map.webp)). Additionally, if you’ve subscribed
  to a feed that has geodata in it, you’ll see a little map icon on its card you can use to map its articles.
- **Feed Discovery:** There is now a [Discover](https://foragd.app/discover) page, where you can browse feeds by
  category or search them to add new subscriptions.
- **Sites Without Feeds:** I’m rolling out support for sites without feeds. Rather than having you need to understand
  HTML/CSS or drive an AI, just send me details of the site and I’ll work out if and how to parse it as a feed. This
  service is part of your subscription.

**UX & UI Improvements:**

- **Tooltips:** More parts of the UI have tooltips to help you understand what an icon/button is/does, or show full-text
  where it has been truncated.
- **Scroll Position Preserved:** I’ve been working on ensuring your scroll position gets preserved when navigating
  between pages, where you’d expect it to.
- **Font Choices:** You can now choose from different font styles for article text. If you prefer serif or sans-serif,
  there are now options for those.
- **Hide Grouped Subscriptions:** You can now opt to have any grouped subscriptions hidden on the subscriptions list
  page, so you’ll only see the group itself.
- **Improved YouTube Integration**: Searching for YouTube for new feeds is improved and results will include both
  channels and playlists that match your search.

**Backend Improvements:**

- More backend optimizations mean even faster loading of home page and list pages.
- Better article enrichment processing. The is most noticeable for feeds like Hackernews and Lobsters, which only
  contain a title and a link. Now, the articles you see will have an image and description as well where found, so you
  can glance through and even filter the articles without having to click each one.

**Links:**

- [Full Changelog](https://foragd.app/changelog)

*Foragd is made with handcrafted code. If you have any questions, feedback, or feature requests, [contact
me!](https://foragd.app/contact).*
