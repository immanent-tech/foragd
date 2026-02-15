+++
title = "How to Find RSS and Atom Feeds for any website"
description = "RSS and Atom are formats that are “hiding in plain sight”. Lots of websites have them, but it might not be obvious how to get them."
created_at = "2026-02-02"
updated_at = "2026-02-11"
image = "/content/images/posts/Ferdinand-Magellan-Portuguese-fleet-departure-ships-wood-September-20-1519.jpg"
+++


<figure>
  <img
    src="/content/images/posts/Ferdinand-Magellan-Portuguese-fleet-departure-ships-wood-September-20-1519.jpg"
    alt="Ferdinand Magellan's fleet" />
  <figcaption>
    Portuguese explorer Ferdinand Magellan's fleet of five ships after their departure from Spain on September 20, 1519; wood engraving, 19th century.
North Wind Picture Archives/Alamy
  </figcaption>
</figure>

## Feed Discovery Was Designed For Machines, Not Humans 🙁

Partly, this is due to the way the RSS specification suggests advertising feeds through an autodiscovery mechanism (see [here](https://www.rssboard.org/rss-autodiscovery) for the technical details). This process is less for humans and more for automation, like your browser, or your feed reader. That was a novel and useful approach back when browsers had integrated feed readers and RSS/Atom were more popular. Nowadays, it makes these formats harder to find and discover.

The good news however is that there are lots of other ways to find good feed sources. A few techniques are listed in this article.

## Technique 1: Use Your Feed Reader

A good feed reader, like [Foragd](https://foragd.app) can utilize the autodiscovery process, along with other sleuthing techniques to find feeds for your favorite sites. So in a lot of cases, it may be as simple as just providing the URL of the site to the feed reader and let it do its magic. No need to parse the site's HTML or scan the content or find a site directory; just enter the URL, and in most cases, 💥 you have your favorite site's content streaming to your feed reader.

ℹ️ **You can use Foragd's [Feed Viewer](https://foragd.app/viewer) to find and parse the feed content of any website.**

## Technique 2: Where's Waldo

A lot of times, the site will stick the feed link in its footer. Sometimes it'll be literal text such as “Feed”, “RSS” or “Atom”. Other times, it'll be the RSS icon, for example one of the following:

<div class="flex mx-auto space-x-4 justify-center">
<img class="flex size-8" src="/content/rss-dark.svg" alt="Typical RSS icon"/>
<img class="flex size-8" src="/content/file-rss-dark.svg" alt="Typical RSS alternative icon"/>
</div>

Such links usually return the raw feed content, so they can be copied and pasted into your feed reader to add them.

## Technique 3: The Old Appender

Most websites are built with a framework and most websites are built with the same frameworks. Most of these frameworks automatically generate feeds for the site. What does this mean? It means most websites have an RSS feed at a canonical URL or address. You can then exploit this to find the feed for any site. In most cases add one of the following onto the end of the site URL:

- `/rss`
- `/feed`
- `/feeds/posts/default`

[Foragd](https://foragd.app) uses this technique when it can't find a feed natively, but you can also check yourself. If the new URL returns feed content, you've found yourself the site's feed!

## Technique 3: Feed Search Engines/Lists

There are a few dedicated search engines for feeds and sites with quality feed links out there you can peruse:

- [feedle.world](https://feedle.world/): A feed search engine and discovery site. Contains a lot of independent bloggers and sources. Try the [random search](https://feedle.world/random) feature 🎲.
- [feedsearch.dev](https://feedsearch.dev/): While targeted for API usage (i.e., built into another app), the site itself will return a human-readable list of feeds for a given site.
- [Feedspot](https://rss.feedspot.com/): A large database of RSS feeds. The categories are a bit dubious and IMO SEO-clickbaity but its possible to find some useful feed links among the lists.
- [RSSHub](rsshub.app): Is both a search engine and tool you can self-host to provide feed links for sites, including generating feed links for sites that don't publish their own. There are [public instances](https://docs.rsshub.app/guide/instances) you use to [browse](https://docs.rsshub.app/routes/) all available feed links for different sites. Quality can be hit-and-miss, so YMMV.

A little bit of search-fu might work if all else fails. In your favorite search engine, try a search like `site:my-favorite.site file:rss`. If you are lucky, the results may include the RSS link for the site.

## Conclusion

So there are a number of ways to find feeds. However, the takeaway in most cases is to just enter the site URL into your feed reader and let it do the sleuthing for you. Most likely you'll hit gold and add another source to your reading collection.
