+++
title = "Foragd October 2026 Update"
page_title = "Foragd RSS/Atom/JSON Feed Reader - October 2026 Update"
description = "A quick summary of updates to Foragd for September 2026"
created_at = "2026-10-15"
updated_at = "2026-10-15"
image = "/images/logo-vertical-light.webp"
author = "Joshua Rich"
slug = "update-oct-2026"
+++

A quick update post summarizing the updates in [Foragd](https://foragd.app) for October 2026. This month has been a lot
more backend work; adding various testing and performance improvements to make Foragd less buggy and more performant.

**What’s new in this update:**

- **Subscription Hues:** Subscriptions now have a distinct hue associated with them, which is used in various parts of
  the interface to add a small visual difference and create associations between subscriptions and articles more easy to
  see.
- **Language filter:** You can now filter by the language of the subscription or article.

**UX & UI Improvements:**

- **Card visual refresh:** The subscription and article cards have had a visual refresh and display more information.
  You can now see indicators for any article filtering you've applied on subscription cards, and whether there are
  articles with geo information. There is a button to take you to the feed's website as well.
  - [Subscription Card Screenshot](/images/screenshots/screenshot-subscription-card.webp).
  - [Article Card Screenshot](/images/screenshots/screenshot-article-card.webp).
- **Reworking imports:** Importing via OPML has been greatly overhauled. You now start an import and it is processed in
  the background. There is a status page you can check on the progress, and new subscriptions will show up as they are
  added and updated. Any import errors are clearer as well.
- **Bulk unsubscribe:** You can now bulk unsubscribe from multiple selected subscriptions on the subscriptions settings
  page.
- **Subscription category management:** When adding or editing a subscription, you can now choose from a set of standard
  categories, as well as adding your own. Using the standard categories can help avoid a categorization explosion and
  typos, as these are consistent and stable across all subscriptions.
- **Grid layout and scroll restore improvements:** the grid layout of subscription/article cards is more robust and your
  scroll position is restored more accurately when navigating back.
- **Mark read while browsing:** If you have "Mark Article Read on View" enabled in the display settings, then browsing
  through articles with the next/previous buttons will automatically mark the articles as read.

**Backend Improvements:**

- Foragd has gotten smarter about fetching images for articles. This should mean less chance of a placeholder image
  being used and a richer display in articles.
- I've been implementing more robust testing of the application, from backend to frontend, to capture more bugs and
  issues before they hit you.

**Links:**

- [Full Changelog](https://foragd.app/changelog)

*Foragd is made with handcrafted code. If you have any questions, feedback, or feature requests, [contact
me!](https://foragd.app/contact).*
