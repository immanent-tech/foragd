+++
title = "Foragd Bot"
description = "Foragd Bot Information"
created_at = "2025-10-07"
updated_at = "2026-02-06"
+++

# Foragd Bot Information

[Foragd.app](https://foragd.app) is a web application service for where users can view RSS, Atom, and other syndication feeds they have subscribed to.

## How does foragd.app access websites?

When users request to subscribe to a new website, they enter a website URL, and the service will attempt to either find the feed URL using auto-discovery (i.e., [RSS auto-discovery](https://www.rssboard.org/rss-autodiscovery)) or use the URL directly if it points to a feed file format.

For websites without a public feed, the service may resort to examining the website sitemap and [Open Graph](https://ogp.me/) data to determine appropriate content to substitute for a feed. For example, the service will attempt to use a [news sitemap](https://developers.google.com/search/docs/crawling-indexing/sitemaps/news-sitemap) if one is available.

The service never performs wholesale scraping of all public content. All requests are targeted to either published RSS and Atom files or other specific content as mentioned above.

Once a user has subscribed, the service will periodically poll the website to find new items in the feed. It respects any configured update intervals specified in the feed (e.g., [syndication extension for RSS](https://web.resource.org/rss/1.0/modules/syndication/)), otherwise will default to hourly fetching of the feed.

## Request Details

The canonical form of the User-Agent header that the service uses for all requests is:

```
Foragd/_version_ (+https://foragd.app/policies/bot)
```

Where `_version_` reflects an internal version of the service.

Please note that it is trivial for malicious users to spoof User-Agents, so the presence of the above User-Agent string does not prove the provenance of, nor indicate that requests are sourced from the service.

## Source Code

The source code for the library that fetches and parses feed content can be found at [github.com/immanent-tech/go-syndication](https://github.com/immanent-tech/go-syndication).

The source code for foragd.app can be found at [github.com/immanent-tech/foragd](https://github.com/immanent-tech/foragd).

## Contact

For any communication, questions or comments about the scraping from the service, please email [bot@immanent.tech](mailto:bot@immanent.tech).
