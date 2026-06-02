# Foragd Help

- [Foragd Help](#foragd-help)
  - [Where to get help](#where-to-get-help)
  - [Terminology](#terminology)
  - [Accounts](#accounts)
  - [How to Use Foragd](#how-to-use-foragd)
    - [Getting Around the Interface](#getting-around-the-interface)
    - [How to Search Articles \& Subscriptions](#how-to-search-articles--subscriptions)
      - [About Search Results](#about-search-results)
    - [Managing Your Subscriptions](#managing-your-subscriptions)
      - [Customising the Subscription Display](#customising-the-subscription-display)
      - [Filtering Articles Within a Subscription](#filtering-articles-within-a-subscription)
      - [Group Subscriptions](#group-subscriptions)
      - [Search Subscriptions](#search-subscriptions)
      - [Email Newsletter Subscriptions](#email-newsletter-subscriptions)
      - [Managing Subscriptions](#managing-subscriptions)
    - [Saving and Viewing Your Favorite Subscriptions and Articles](#saving-and-viewing-your-favorite-subscriptions-and-articles)
    - [Search Operators \& Filtering Guide](#search-operators--filtering-guide)
    - [Using Keyboard Shortcuts (on Desktops)](#using-keyboard-shortcuts-on-desktops)
  - [Policies and Terms of Service](#policies-and-terms-of-service)

## Where to get help

- This document.
- Email us at [support@immanent.tech](mailto:support@immanent.tech).

## Terminology

- **Subscription** is a source of articles created by the user. The articles may come from different sources, such as a
  single website, a group of other subscriptions or a configured search. These can also be referred to individually as:
  - **Feed Subscription**: a subscription to a single website (i.e., an RSS/Atom feed for a particular website).
  - **Group Subscription**: a subscription that aggregates the articles from other subscriptions.
  - **Search Subscription**: a subscription created from a particular set of search terms.
- **Article**: is a single item from a subscription (i.e., an article, blog post, etc.).

## Accounts

- You can manage your account at [Settings->Account](/user/settings#account).
- You can change your plan level or cancel anytime.
- Cancelling a plan occurs at your next billing date. Until then, you can continue to use your plan. You can also
  reverse a cancellation during this period if you change your mind.

**Note:** Foragd uses [Paddle](https://www.paddle.com/) as our payments processor. You will be taken to a hosted Paddle
payment page for managing your subscriptions and payments.

## How to Use Foragd

### Getting Around the Interface

Use the sidebar (desktops, tablets) or bottom dock (mobile) to navigate between the Home, Subscriptions, Articles, or
Favorites pages.

### How to Search Articles & Subscriptions

Use the global search in the header bar to search for anything. It will search across all Subscriptions and Articles,
offering both as suggestions. Additionally certain keywords will show action results (for example, searching `add` will
show actions for adding subscriptions). Choose a result to go directly to that article or subscription, an action to
perform that action, or hit enter/return to get a full set of results.

Use the advanced search (filter icon at the right of the search bar) to filter by subscription, category, author, date,
and status. See also [filtering](#filtering-articles-within-a-subscription) for operators that can be used in the text fields.

#### About Search Results

- Search will preference articles from your favorite subscriptions.
- Search will preference articles that have been published/updated closer to today's date.

### Managing Your Subscriptions

#### Customising the Subscription Display

You can customize any subscription, by changing its name or adding/removing categories. To customize a subscription,
choose **Edit Subscription** from the context menu of the Subscription card:

![Screenshot of subscription customization](/content/screenshots/screenshot-subscription-customisation.png)

Categories allow you to easily group and filter your subscriptions. Where possible, some suggested categories will be
presented as auto-complete options in the **Add Categories** input. These will be taken from the most common categories
on Articles within the feed.

#### Filtering Articles Within a Subscription

You can filter the articles in a subscription by text, authors, or categories. To adjust filters, edit the subscription
and enter your filter terms in the appropriate inputs:

![Screenshot of subscription filtering options](/content/screenshots/screenshot-subscription-article-filters.png)

See [filtering](#filtering-articles-within-a-subscription) for usage.

**Note:** article filters are applied globally, meaning all searches, views and any group/search Subscriptions you
create will have the subscription article filters applied.

#### Group Subscriptions

You can add a _Group Subscription_ which aggregates all the Articles from two or more Feed Subscriptions. This is useful
where you subscribe to a number of feeds that have similar content, like Android news sites. When creating a Group
Subscription, use the provided search input to filter your existing subscriptions for the ones you want to add:

![Screenshot of creating a Group Subscription](/content/screenshots/screenshot-add-group-subscription.png)

To add a Group Subscription:

- Search for _Add_ in the global search and choose the **Add A Group Subscription** action.
- On the [Subscriptions](/list/subscriptions) page, select **Add A Group Subscription** from the _Actions_ menu.

#### Search Subscriptions

Any [search](#how-to-search-articles--subscriptions) can also be made into a Search Subscription. This is useful for
keeping track of particular keywords or content across any number of your feed subscriptions.

To add a Group Subscription:

- Search for _Add_ in the global search and select the **Add A Search Subscription** action.
- On the [Subscriptions](/list/subscriptions) page, select **Add A Search Subscription** from the _Actions_ menu.

![Screenshot of creating a Search Subscription](/content/screenshots/screenshot-add-search-subscription.png)

#### Email Newsletter Subscriptions

You can subscribe to your favorite newsletters from within Foragd. This allows you to read newsletters alongside your
RSS feeds without cluttering your inbox.

First, you will need to create your unique Foragd email address that you will use to subscribe to any newsletters.

- Navigate to [Settings](/settings)->Account and scroll down to *Subscribe to Email Newsletters*.
- Click on the *Generate Email Address* button to generate a unique email.
- The page will update with the new unique email.

Once you’ve generated your email address, use it when subscribing to email newsletters. The received emails will be
automatically grouped under a new subscription from the sender email. You can then customize it further (add a picture,
nickname, or even filter received emails) as with any other subscription.

#### Managing Subscriptions

You can manage your subscriptions at [Settings->Subscriptions](/user/settings#subscriptions). This page provides a way
to edit, mark and manage subscriptions both individually and in bulk. It also shows how many subscriptions you have in
total (and what your account limits on subscriptions are).

### Saving and Viewing Your Favorite Subscriptions and Articles

You can mark any subscription or article as a favorite. All favorites can be found on the [favorites](/list/favorites)
page. On the subscriptions list page, you can filter to show only favorites.

While Foragd does not retain all feed articles forever, marking an article as a favorite **will** ensure it is
kept indefinitely. Foragd makes a copy of the article and stores it specially for you.

### Search Operators & Filtering Guide

By default words are matched with **OR** (i.e., cats dogs will match cats **OR** dogs). The following operators can be
used for more refined searches:

- `+` signifies AND operation (i.e., food **AND** beverages).
- `|` signifies OR operation (i.e., food **OR** beverages).
- `-` negates the word (i.e., **NOT** samsung)
- `""` represents an exact phrase match (i.e., `"galaxy watch"`).
- `\*` at the end of a word indicates a prefix match (i.e., `bird\*` will match bird, birding and birds).

Some real examples:

| Goal                                                                 | Query                |
| -------------------------------------------------------------------- | -------------------- |
| AI news, but not about ChatGPT                                       | `AI -chatgpt`        |
| Find articles about "machine learning" (phrase)                      | `"machine learning"` |
| Find articles that are only about python and that are also tutorials | `+python +tutorial`  |

All operators can be combined, for e.g., `+python +tutorial "machine learning" -chatgpt`.

### Using Keyboard Shortcuts (on Desktops)

The following shortcut keys are available on desktop:

| Key Combo | Action                                                                                                        |
| --------- | ------------------------------------------------------------------------------------------------------------- |
| `Alt+k`   | activates the global search input                                                                             |
| `Alt+h`   | navigates to [Home](/home)                                                                                    |
| `Alt+s`   | navigates to [Subscriptions](/list/subscriptions)                                                             |
| `Alt+a`   | navigates to [Articles](/list/articles)                                                                       |
| `Alt+f`   | navigates to [Favorites](/list/favorites)                                                                     |
| `Alt+x`   | activates the actions menu (on [Subscriptions](/list/subscriptions) or [Articles](/list/articles) list pages) |

## Policies and Terms of Service

- [Acceptable Use Policy](/policies/acceptable-use).
- [Terms of Service](/policies/tos).
- [Privacy Policy](/policies/privacy).
