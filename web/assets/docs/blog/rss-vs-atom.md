+++
title = "RSS vs Atom"
page_title = "RSS vs Atom Feed Viewer Formats"
description = "RSS and Atom both define a feed for a website. This article looks into the differences and whether it even matters."
created_at = "2026-07-06"
updated_at = "2026-07-06"
image = "/content/images/blog/rss-vs-atom-hero.webp"
author = "Joshua Rich"
slug = "rss-vs-atom"
+++

## RSS vs Atom, the TL;DR

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

## The Differences

First off, Mark Nottingham (who authored the Atom spec) has [run some numbers](https://mnot.net/blog/2026/feed-survey)
that show that RSS is implemented more often than Atom. As Mark notes, this likely is not meaningful as a decision key
on which to implement.

Atom is much more expressive in terms of the structure that defines items in the feed. RSS has less rigidity, but
ultimately both cover all the canonical structure you'd expect for items, like titles, descriptions, content, and media
elements. Additionally, both support extensions that provide a way to add functionality to their base layouts and often
the same extensions work in both formats due to them sharing the XML base format. Wikipedia has a good quick [comparison
of the difference in structure](https://en.wikipedia.org/wiki/Web_feed#RSS_compared_with_Atom) between the two.

## Which Should You Use?

**As an end user:** If you have a choice, pick either as nearly all good feed readers, like
[Foragd](https://foragd.app), support both formats.

**As a developer:** If you are looking to add a feed to your project, use whichever format your framework or service
readily provides. For end-users, it won't matter as the app or service they use to consume your feed will support both
formats and the feature-set of each format is nearly identical.

*Shameless plug: If you are a Go developer, you might be interested in
[go-syndication](https://github.com/immanent-tech/go-syndication), which provides reading and writing for RSS, Atom,
JSONFeed and a number of extensions for these formats.*

If you want the full details, both specifications are surprisingly easy to read (see references below).

## References

- <https://en.wikipedia.org/wiki/Web_feed>
- <https://en.wikipedia.org/wiki/RSS>
- <https://en.wikipedia.org/wiki/Atom_(standard)>
- <https://www.rssboard.org/rss-specification>
- <https://www.rfc-editor.org/info/rfc4287/>
