+++
title = "How to Find RSS and Atom Feeds for any website"
page_title = "Tips and tricks to find RSS and Atom pages on the internet"
description = "RSS and Atom are formats that are “hiding in plain sight”. Lots of websites have them, but it might not be obvious how to get them."
created_at = "2026-02-02"
updated_at = "2026-04-28"
image = "/content/images/blog/Ferdinand-Magellan-Portuguese-fleet-departure-ships-wood-September-20-1519.webp"
author = "Joshua Rich"
+++

<script id="finding-feeds-faq" type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "HowTo",
  "name": "How to Find RSS and Atom Feeds for Any Website",
  "description": "Several techniques for finding RSS and Atom feeds on any website.",
  "step": [
    {
      "@type": "HowToStep",
      "name": "Use your feed reader",
      "text": "Paste the website URL directly into your feed reader. Most good feed readers like Foragd will automatically discover the feed using autodiscovery and other techniques."
    },
    {
      "@type": "HowToStep",
      "name": "Look in the website footer",
      "text": "Many websites display RSS or Atom feed links in their footer as text ('RSS', 'Feed', 'Atom') or as the orange RSS icon."
    },
    {
      "@type": "HowToStep",
      "name": "Try common feed URL paths",
      "text": "Append /feed, /rss, or /feeds/posts/default to the website's base URL and check if feed content is returned."
    },
    {
      "@type": "HowToStep",
      "name": "Use a feed search engine",
      "text": "Try feedle.world, feedsearch.dev, Feedspot, RSSHub, or Kagi Small Web to search for feeds by site or topic."
    }
  ]
}
</script>

<figure>
  <img
    src="/content/images/blog/Ferdinand-Magellan-Portuguese-fleet-departure-ships-wood-September-20-1519.webp"
    alt="Ferdinand Magellan's fleet" />
  <figcaption>
    Portuguese explorer Ferdinand Magellan's fleet of five ships after their departure from Spain on September 20, 1519; wood engraving, 19th century.
North Wind Picture Archives/Alamy
  </figcaption>
</figure>

## Feed Discovery Was Designed For Machines, Not Humans 🙁

Partly, this is due to the way the RSS specification suggests advertising feeds through an autodiscovery mechanism (see
[here](https://www.rssboard.org/rss-autodiscovery) for the technical details). This process is less for humans and more
for automation, like your browser, or your feed reader. That was a novel and useful approach back when browsers had
integrated feed readers and RSS/Atom were more popular. Nowadays, it makes these formats harder to find and discover.

The good news however is that there are lots of other ways to find good feed sources. A few techniques are listed in
this article.

## Technique 1: Use Your Feed Reader

Modern feed readers, like [Foragd](https://foragd.app?utm_source=blog), will auto-detect the feed given a website URL.
They utilize the autodiscovery process, along with other sleuthing techniques to find feeds for your favorite sites. So
in a lot of cases, it may be as simple as just providing the URL of the site to the feed reader and let it do its magic.
No need to parse the site’s HTML or scan the content or find a site directory; just enter the URL, and in most cases, 💥
you have your favorite site’s content streaming to your feed reader.

ℹ️ **You can use Foragd’s [Feed Viewer](/viewer?utm_source=blog) to find and parse the feed content of any website.**

## Technique 2: Where’s Waldo

Many websites display their RSS feed link in the footer as text or an RSS icon — look for the words 'Feed', 'RSS', or
'Atom', or the orange RSS icon. For example one of the following:

<div class="flex mx-auto space-x-4 justify-center not-prose">
<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-rss size-8 flex text-orange-500 fill-orange-500 stroke-orange-500"><path stroke="none" d="M0 0h24v24H0z" fill="none" /><path d="M4 19a1 1 0 1 0 2 0a1 1 0 1 0 -2 0" /><path d="M4 4a16 16 0 0 1 16 16" /><path d="M4 11a9 9 0 0 1 9 9" /></svg>
<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="icon icon-tabler icons-tabler-outline icon-tabler-file-rss size-8 flex  text-orange-500 fill-orange-500 stroke-orange-500""><path stroke="none" d="M0 0h24v24H0z" fill="none" /><path d="M14 3v4a1 1 0 0 0 1 1h4" /><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2" /><path d="M12 17a3 3 0 0 0 -3 -3" /><path d="M15 17a6 6 0 0 0 -6 -6" /><path d="M9 17h.01" /></svg>
</div>

Such links usually return the raw feed content, so they can be copied and pasted into your feed reader to add them.

## Technique 3: The Old Appender

Despite the web being wonderfully diverse, the majority of websites use a handful of frameworks behind the scenes, and
these have predictable URLs where their feeds are located. What does this mean? It means most websites have an RSS feed
at a canonical URL or address. You can then utilise this to find the feed for any site. In most cases add one of the
following onto the end of the site URL:

- `/rss`
- `/feed`
- `/feeds/posts/default`

Note that [Foragd](https://foragd.app?utm_source=blog) uses a bunch of techniques when it can’t find a feed natively, so
that you don't need to use trial-and-error to find the feed URL yourself!

### A Few Well-known URLs For Specific Sites

#### Reddit

For any sub-reddit, just append `.rss` on the end of the URL. For example:
[https://reddit.com/r/rss/.rss](https://reddit.com/r/rss/.rss).

#### Tumblr

A Tumblr blog usually has an RSS feed at `/rss` on the end of the URL.

#### Medium

Medium [helpfully
documents](https://help.medium.com/hc/en-us/articles/214874118-Using-RSS-feeds-of-profiles-publications-and-topics) how
to generate URLs to get feeds for authors, publications and even topics.

## Technique 4: Feed Search Engines/Lists

There are a few dedicated search engines for feeds and sites with quality feed links out there you can peruse:

- [feedle.world](https://feedle.world/): A feed search engine and discovery site. Contains a lot of independent bloggers
  and sources. Try the [random search](https://feedle.world/random) feature 🎲.
- [feedsearch.dev](https://feedsearch.dev/): While targeted for API usage (i.e., built into another app), the site
  itself will return a human-readable list of feeds for a given site.
- [Feedspot](https://rss.feedspot.com/): A large database of RSS feeds. The categories are a bit dubious and IMO
  SEO-clickbaity but its possible to find some useful feed links among the lists.
- [RSSHub](https://rsshub.app): Is both a search engine and tool you can self-host to provide feed links for sites, including
  generating feed links for sites that don’t publish their own. There are [public
  instances](https://docs.rsshub.app/guide/instances) you use to [browse](https://docs.rsshub.app/routes/) all available
  feed links for different sites. Quality can be hit-and-miss, so YMMV.
- [Kagi Small Web](https://kagi.com/smallweb): is an old-school webring of independent sites. You can search or navigate
  by site and topics to find independent blogs and sources.

A little bit of search-fu might work if all else fails. In your favorite search engine, try a search like
`site:my-favorite.site file:rss`. If you are lucky, the results may include the RSS link for the site.

## Conclusion

To find RSS and Atom feeds for any website, try these approaches in order: paste the site URL directly into your feed
reader and let it auto-discover the feed; look for RSS or feed links in the site's footer; try appending `/feed`,
`/rss`, or `/feeds/posts/default` to the site URL; or use a dedicated feed search engine like
[feedle.world](https://feedle.world/), [feedsearch.dev](https://feedsearch.dev/), or [RSSHub](https://rsshub.app). In
most cases, a good feed reader like [Foragd](https://foragd.app?utm_source=blog) will handle discovery automatically.

If you want to try out using RSS and Atom feeds, you can start a free trial of [Foragd](https://foragd.app?utm_source=blog) and start
gathering your own collection of topics, news, and opinions!

License: [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
