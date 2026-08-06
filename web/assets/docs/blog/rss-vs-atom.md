+++
title = "RSS vs Atom"
page_title = "RSS vs Atom Feed Viewer Formats"
description = "RSS and Atom both define a feed for a website. This article looks into the differences and whether it even matters."
created_at = "2026-07-06"
updated_at = "2026-08-06"
image = "/content/images/blog/rss-vs-atom-hero.webp"
author = "Joshua Rich"
slug = "rss-vs-atom"
+++

## RSS Vs Atom, the TL;DR

RSS and Atom are both file formats that define a website's feed. Feed viewers fetch and parse these formats to generate
a list of articles or posts you can view in a feed viewer like [Foragd](https://foragd.app).

**The TL;DR:** as a user wanting to read content from feeds in your favorite reader, use either. As a developer, use
whichever is readily available in your framework.

Read on for more details about both formats, their differences, and similarities and some nuances about their use.

<figure>
  <img
    src="/content/images/blog/rss-vs-atom-hero.webp"
    alt="A visual representation of RSS and Atom formats merging into a common structure" />
</figure>

## The Similarities

RSS and Atom feeds are both [XML](https://en.wikipedia.org/wiki/XML) file formats that contain a structure for defining
links, or in some cases, full content of posts for a website. Generally, they are used for blog posts or articles, but
work well for any kind of syndicated content that updates on a regular basis. So breaking news, new items or listings,
status updates or even new comments could be represented as one of these feeds.

Simplified, they have the following structure:

<figure>
  <img
    src="/content/images/blog/feed-format-structure.webp"
    alt="Diagram showing the common structure of feed formats, with the container, metadata and individua items" />
</figure>

Items are usually sorted by their creation date. The number of items is not capped or limited, but generally most feed
publishers will expose a rolling number of items, with older items being removed after some period.

Both formats include publisher details that make the files self-contained, allowing them to be shared or published
elsewhere than the source website, and retain the necessary details to link back to that source.

Being self-contained and capping the number of items  is important, as these formats are literally downloaded as a file
to your feed reader. They aren't a "stream" of data that a server sends to a client. They are a full file, like a word
document, you download and parse. So keeping the size constrained and ensuring it contains relevant metadata and sources
is vital.

Unlike a web page, these formats do not contain minimal formatting and other than the structure shown above, its left to
the end consumer to format them as they wish. This gives you a lot of flexibility in how to display feeds, though most
feed readers default to an inbox-like display. IMO, treating feeds like email is a bad analogy and [Foragd](/) is
[intentionally different](/about).

## The Differences

First off, Mark Nottingham (who authored the Atom spec) has [run some numbers](https://mnot.net/blog/2026/feed-survey)
that show that RSS is implemented more often than Atom. As Mark notes, this likely is not meaningful as a decision key
on which to implement. Popularity is not a good indicator of superiority.

Atom is much more expressive in terms of the structure that defines items in the feed. RSS has less rigidity, but
ultimately both cover all the canonical structure you'd expect for items, like titles, descriptions, content, and media
elements. Additionally, both support extensions that provide a way to add functionality to their base layouts and often
the same extensions work in both formats due to them sharing the XML base format. Wikipedia has a good quick [comparison
of the difference in structure](https://en.wikipedia.org/wiki/Web_feed#RSS_compared_with_Atom) between the two.

## Which Should You Use?

**As an end user:** If you have a choice, pick either as nearly all good feed readers, like
[Foragd](https://foragd.app), support both formats. As they provide near identical features, the choice as a consumer of
the feed is irrelevant. Your feed read will control how they are displayed, so focus on finding a feed reader that has
the features and layout you like.

**As a developer:** If you are looking to add a feed to your project, use whichever format your framework or service
readily provides. Minimal implementation effort should be your focus, as the provided structure of either is
functionally equivalent, and any additional features you can get through common extensions. For your end-users, it won't
matter as the app or service they use to consume your feed will support both formats and is ultimately in control of how
your feed will be displayed.

*Shameless plug: If you are a Go developer, you might be interested in
[go-syndication](https://github.com/immanent-tech/go-syndication), which provides reading and writing for RSS, Atom,
JSONFeed, and a number of extensions for these formats.*

If you want the full details, both specifications are surprisingly easy to read (see references below).

Getting started with RSS and Atom feeds? Foragd is free to try for 30 days — no commitment needed. Sign up.

## References

- <https://en.wikipedia.org/wiki/Web_feed>
- <https://en.wikipedia.org/wiki/RSS>
- <https://en.wikipedia.org/wiki/Atom_(standard)>
- <https://www.rssboard.org/rss-specification>
- <https://www.rfc-editor.org/info/rfc4287/>

License: [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
