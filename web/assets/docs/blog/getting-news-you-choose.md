+++
title = "Getting News you Choose: A Selection of News Feeds"
page_title = "Getting News you Choose: A Selection of News Feeds"
description = "There are lots of places to get your news online. With feeds however, you get more control; you choose the sources and can filter the articles as you like."
created_at = "2026-08-24"
updated_at = "2026-08-24"
image = "/content/images/blog/the-yellow-press.webp"
author = "Joshua Rich"
slug = "getting-news-you-choose"
+++

Social media and traditional news publishing websites are the two main ways people get their news and current affairs
updates today. But there's another option. RSS and Atom feeds let you not only collect your news updates, but take real
control over them. With a good feed reader, you can gather, filter, and search across sources far more easily than
relying on individual sites and social networks. You also escape the algorithm deciding what you see, the ads mixed into
your timeline, and the tracking of what you read.

<figure>
  <img
    src="/content/images/blog/the-yellow-press.webp"
    alt="The Yellow Press, illustration from 1910 depicting William Randolph Hearst as a jester tossing newspapers with headlines such as 'Appeals to Passion, Venom, Sensationalism, Attacks on Honest Officials, Strife, Distorted News, Personal Grievance, [and] Misrepresentation' to a crowd of eager readers" />
  <figcaption>
    The Yellow Press by Louis Glackens, 1910. From Puck, v. 68, no. 1754. Sensationalist headlines aren't a new problem, they're just algorithmically amplified today. Feeds hand curation back to you.
  </figcaption>
</figure>

## A Curated Selection of Sources

To get started, you'll need a good list of sources. Here is a curated selection of news sources and their feeds (and
with links to directly subscribe in [Foragd](https://foragd.app)).

### Worldwide News

- [BBC News](https://www.bbc.co.uk)
  - UK-based online news source.
  - 🔗 [Feed URL](https://feeds.bbci.co.uk/news/rss.xml)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://feeds.bbci.co.uk/news/rss.xml)

- [SBS News](https://www.sbs.com.au/news)
  - Australian-based news source.
  - 🔗 [Feed URL](https://sbs.com.au/news/feed)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://sbs.com.au/news/feed)

- [Global Voices](https://globalvoices.org/feed/)
  - Citizen media stories from around the world.
  - 🔗 [Feed URL](https://globalvoices.org/feed/)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://globalvoices.org/feed/)

### Finance

- [Yahoo Finance](https://finance.yahoo.com)
  - Finance and business news.
  - 🔗 [Feed URL](https://finance.yahoo.com/news/rssindex)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://finance.yahoo.com/news/rssindex)

### Science

- [New Scientist](https://www.newscientist.com/)
  - Science news and science articles.
  - 🔗 [Feed URL](https://www.newscientist.com/feed/home/)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://www.newscientist.com/feed/home/)

- [Live Science](https://www.livescience.com)
  - Science news and related updates.
  - 🔗 [Feed URL](https://www.livescience.com/feeds.xml)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://www.livescience.com/feeds.xml)

### Tech

- [Ars Technica](https://arstechnica.com/)
  - News and reviews, covering IT, AI, science, space, health, gaming, cybersecurity, tech policy, computers, mobile
    devices, and operating systems.
  - 🔗 [Feed URL](http://feeds.arstechnica.com/arstechnica/index/)
  - ➕ [Subscribe in Foragd](/subscription/add?url=http://feeds.arstechnica.com/arstechnica/index/)

- [Rest of the World](https://restofworld.org/)
  - Global tech news outside of the western world.
  - 🔗 [Feed URL](https://restofworld.org/feed/latest)
  - ➕ [Subscribe in Foragd](/subscription/add?url=https://restofworld.org/feed/latest)

## Choosing a Feed Reader

Before subscribing to everything above, it's worth picking a reader that fits how you actually read. Look for:

- **Filtering**: the ability to mute keywords or highlight topics you care about.
- **Multiple device support/sync**: if you read on the go, a reader with a solid phone experience matters, as well as
  the ability to remember what you've read and what you've yet to read across devices.
- **Search**: across both current and archived articles, not just what's unread.
- **OPML support** the standard file format for importing and exporting your subscription list, so you're never locked
  into one app.

[Foragd](https://foragd.app) covers all of these, but the same principles apply whatever reader you choose.

## Fully Customised News Source with Google News

The above sites are great for keeping up to date across a wide variety of topics. But you'll still get articles that
might not be of interest to you. Fortunately, a good feed reader like Foragd can help by applying article filtering (see
the [blog post](/blog/article-filtering-in-foragd) for a howto and details). That's the power of RSS/Atom as your
source; you are in control of how it gets displayed to you.

But for a truly customised source, there's another trick: Google News, which has built-in RSS feeds for any query you
can enter on the site.

You might already be familiar with [news.google.com](https://news.google.com) and may already be using it. Hidden within
this site is native RSS support, meaning you can subscribe to news directly in your feed reader. What's even more
impressive, it supports basically any search and filtering you might do on the website itself, allowing you to target
exactly the news you are interested in. You can even create multiple feeds for different searches and subscribe to them
all in your feed reader.

Beyond plain text search, Google News supports a number of search operators. If you are a Foragd user, these will be
familiar (Foragd uses a similar, but not the same syntax for many search operators). Some key operators:

- Quotes, around words `" "` turns them into a phrase match (i.e., match exactly **"buttered popcorn"** as opposed to
  **buttered** and/or **popcorn**).
- `AND` requires that the terms either side **must** appear in results. i.e., **openai AND revenue**.
- `OR` on the other hand, makes the terms optional. i.e., **Telsa OR "electric car"**.
- `intitle:` matches specifically in the website source's title.
- `source:` or `site:` specify a particlar website source. i.e., `site:reuters.com`.
- `when:` defines a date range for the results. e.g., `when:24h` means last 24 hours.

These work on their own, but they're most powerful combined. For example:

    site:reuters.com when:7d "interest rates"

This returns only Reuters articles from the last 7 days that mention "interest rates"; a feed you could never build
with keyword search alone.

### Building a Custom Feed URL by Hand

If your reader doesn't support Google News natively, you can build the feed URL yourself:

1. On [Google News](https://news.google.com), search for your what you want, using whichever operators you like.
2. On the results page, grab the URL from your browser address bar.
3. Remove everything up to the `q=`. For example, for this URL: **https://news.google.com/search?q=openai%20revenue%20OR%20earnings&hl=en-AU&gl=AU&ceid=AU%3Aen**, we strip it back to **q=openai%20revenue%20OR%20earnings&hl=en-AU&gl=AU&ceid=AU%3Aen**.
4. Prepend `https://news.google.com/rss/search?` to it. So it becomes
   **https://news.google.com/rss/search?q=openai%20revenue%20OR%20earnings&hl=en-AU&gl=AU&ceid=AU%3Aen**.
5. You now have a custom Google News RSS feed URL. Use it to subscribe in your feed reader.

You can create as many different news feeds as you want, filtering to specific topics and/or news sources.

> **Side Note:** [Foragd](https://foragd.app) has native support for Google News feeds and makes this process much easier.
> Just enter your search terms, verify the results, then hit subscribe without any "URL surgery".

## Moving Your Feeds Between Readers

Once you've built up a list of sources, you'll likely want to keep it even if you switch apps. Almost every feed reader
supports OPML import and export. Its a simple file format that lists all your subscriptions. Before committing to a
reader long-term, check that it supports OPML export, so your curated list is never trapped in one tool.

## Get Informed

Quality journalism still exists, and you can access plenty of it outside the big social platforms. What's more, RSS
gives you real control over that content; the ability to search, favourite, and read it all in one place, alongside
every other site and blog you follow.

Get started with reading news on your terms in Foragd. It's free to try for 30 days — no commitment needed. [Sign
up](/signup?utm_source=blog).

License: [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
