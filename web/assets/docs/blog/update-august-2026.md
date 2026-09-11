+++
title = "Foragd RSS/Atom/JSON Feed Reader - August 2026 Update"
page_title = "Foragd RSS/Atom/JSON Feed Reader - August 2026 Update"
description = "A quick summary of updates to the Foragd RSS/Atom/JSON in August 2026"
created_at = "2026-08-18"
updated_at = "2026-08-18"
image = "/content/logo-vertical-light.webp"
author = "Joshua Rich"
slug = "update-august-2026"
+++

A quick update post summarizing the updates in [Foragd](https://foragd.app) for August 2026.

**What’s new in this update:**

- **Global Article Filters:** In addition to per-subscription article filtering, you can now define filters that will
  apply across all subscriptions.
- **Previous/Next Article Buttons**: When viewing an article, there are now buttons that allow you to go the next or
  previous article quickly, without needing to navigate back to the lists.
- **Remembering Scroll Position**: I’m implementing better logic to remember your scroll position when navigating back
  to the lists pages.

**UX & UI Improvements:**

- **Typography:** Improved fonts. Foragd uses [Alegreya](https://fonts.google.com/specimen/Alegreya) as the default
  article font. [Inter](https://fonts.google.com/specimen/Inter) is used for labels and controls. I’m looking into
  adding the ability to choose from a bunch of font sets as well as adjusting the size for individual customization of
  the article font.
- **Themes:** I’ve reworked all available themes, making sure contrast and other issues are resolved. Dark themes in
  particular should have more overall contrast.
- **Better Email Newsletters:** I’ve reworked email newsletter layouts to remove some of the more janky quirks of
  converting email HTML to regular HTML.

**Backend Improvements:**

- Behind the scenes, when the backend fetches feeds, it now tries to enrich feeds/items that lack details, like only
  summaries or no images.
- Search performance and relevance improvements.

**Sale Pricing**

I’m running sale pricing at the moment:

- Annual USD $36 (was $59), paid annually.
- Monthly USD $5 (was $7.50), paid monthly.

The trial period is now 30 days as well, with no need to enter payment details until the trial period is finished.

**Links:**

- [Full Changelog](https://foragd.app/changelog)

*Foragd is made with handcrafted code. If you have any questions, feedback, or feature requests, [contact
me!](https://foragd.app/contact).*
