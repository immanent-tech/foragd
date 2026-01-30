<!--
 Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
 SPDX-License-Identifier: 	AGPL-3.0-or-later
-->

<div align="center">

  <img src="/web/assets/logo-color.svg" alt="logo" width="200" height="auto" />
  <h1>Foragd</h1>
  <p>
    A beautiful, web based, online feed reader.
  </p>
  <p>
    Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop.
  </p>

<!-- Badges -->
<p>
  <a href="https://github.com/immanent-tech/foragd/graphs/contributors">
    <img src="https://img.shields.io/github/contributors/immanent-tech/foragd" alt="contributors" />
  </a>
  <a href="">
    <img src="https://img.shields.io/github/last-commit/immanent-tech/foragd" alt="last update" />
  </a>
  <a href="https://github.com/immanent-tech/foragd/network/members">
    <img src="https://img.shields.io/github/forks/immanent-tech/foragd" alt="forks" />
  </a>
  <a href="https://github.com/immanent-tech/foragd/stargazers">
    <img src="https://img.shields.io/github/stars/immanent-tech/foragd" alt="stars" />
  </a>
  <a href="https://github.com/immanent-tech/foragd/issues/">
    <img src="https://img.shields.io/github/issues/immanent-tech/foragd" alt="open issues" />
  </a>
  <a href="https://github.com/immanent-tech/foragd/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/immanent-tech/foragd.svg" alt="license" />
  </a>
</p>

<h4>
    <a href="https://foragd.app/">Homepage</a>
  <!-- <span> · </span> -->
    <!-- <a href="https://github.com/Louis3797/awesome-readme-template">Documentation</a>
  <span> · </span>
    <a href="https://github.com/Louis3797/awesome-readme-template/issues/">Report Bug</a>
  <span> · </span>
    <a href="https://github.com/Louis3797/awesome-readme-template/issues/">Request Feature</a>
  </h4> -->
</div>

<br />

<!-- Table of Contents -->
# :notebook_with_decorative_cover: Table of Contents

- [:notebook\_with\_decorative\_cover: Table of Contents](#notebook_with_decorative_cover-table-of-contents)
  - [:star2: About the Project](#star2-about-the-project)
    - [What Makes Foragd Different?](#what-makes-foragd-different)
    - [:camera: Screenshots](#camera-screenshots)
    - [:space\_invader: Tech Stack](#space_invader-tech-stack)
    - [:dart: Features](#dart-features)
  - [:toolbox: Getting Started](#toolbox-getting-started)
    - [:bangbang: Prerequisites](#bangbang-prerequisites)
    - [:gear: Installation](#gear-installation)
    - [:test\_tube: Running Tests](#test_tube-running-tests)
    - [:running: Run Locally](#running-run-locally)
    - [:triangular\_flag\_on\_post: Deployment](#triangular_flag_on_post-deployment)
  - [:eyes: Usage](#eyes-usage)
  - [:wave: Contributing](#wave-contributing)
    - [:scroll: Code of Conduct](#scroll-code-of-conduct)
  - [:warning: License](#warning-license)
  - [:handshake: Contact](#handshake-contact)
  - [:gem: Acknowledgements](#gem-acknowledgements)

<!-- About the Project -->
## :star2: About the Project

Foragd is an online, web-based feed reader for all syndication formats (RSS, Atom, JSONFeed).

### What Makes Foragd Different?

- **Focused on reading content, not tracking totals:** not trying to shoehorn feeds into an email-like interface and not showing unread counts. Straightforward homepage, subscription, and article views.
- **Powerful Search:** fast and powerful searching. Find that article mentioning that thing from that site a while back, easily.
- **Easy Filtering:** easily filter articles within a subscription by keyword, phrase, category, or author. No complex filter building, just easy `+/-` operators. For example: `alcoholic drinks + -"rum based" +daiquiri`

<!-- Screenshots -->
### :camera: Screenshots

<div align="center">
  <img src="/web/content/screenshots/home-desktop-mobile.png" alt="Home page on desktop and mobile" />
</div>

<!-- TechStack -->
### :space_invader: Tech Stack

<details>
  <summary>Server</summary>
  <ul>
    <li><a href="https://go.dev/">Golang</a></li>
    <li><a href="https://go-chi.io">Chi</a></li>
    <li><a href="https://tailwindcss.com/">TailwindCSS</a></li>
    <li><a href="https://daisyui.com">Daisy UI</a></li>
  </ul>
</details>

<details>
<summary>Database</summary>
  <ul>
    <li><a href="https://elastic.co/">Elasticsearch</a></li>
  </ul>
</details>

<details>
<summary>Backend</summary>
  <ul>
    <li><a href="https://auth0.com/">Auth0</a></li>
    <li><a href="https://stripe.com/">Stripe</a></li>
    <li><a href="https://resend.com/">Resend</a></li>
  </ul>
</details>

<!-- Features -->
### :dart: Features

- **Mobile and Desktop Friendly.** Foragd is a web based online app. It works in any browser on any device, anywhere.
- **Article Filtering.** Filter articles in subscriptions by text/phrase, category or authors, with easy to use operators.
- **Content Comes First.** Simple UI. Big images. Beautiful typography. Let the content shine.
- **Powerful Search.** Full-text search across subscriptions and articles. Quickly access subscriptions and perform actions from the search bar.
- **Subscription customisation.** Set a nickname for your subscriptions. Add your own categories to easily group and find similar content.
- **Subscription and article favorites.** Mark subscriptions and articles as favorites, to quickly access them later.
- **Group subscriptions.** Combine multiple subscriptions to present a unified view of articles from any of them. Make it easy to keep up with similar posts across different sources.
- **Search subscriptions.** Use the powerful search to find what you need. Save the search terms as a search subscription to always find new articles that match. Great for keeping track of news or topics across multiple subscriptions.

<!-- Getting Started -->
## 	:toolbox: Getting Started

<!-- Prerequisites -->
### :bangbang: Prerequisites

- Podman/Docker.
- Elasticsearch.
- Auth0.
- Stripe.
- Resend.
- GCP.

<!-- Installation -->
### :gear: Installation

TBA.

<!-- Running Tests -->
### :test_tube: Running Tests

TBA.

<!-- Run Locally -->
### :running: Run Locally

TBA.

<!-- Deployment -->
### :triangular_flag_on_post: Deployment

TBA.

<!-- Usage -->
## :eyes: Usage

TBA.

<!-- Contributing -->
## :wave: Contributing

<a href="https://github.com/immanent-tech/foragd/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=immanent-tech/foragd" />
</a>

Contributions are always welcome!

See `contributing.md` for ways to get started.

<!-- Code of Conduct -->
### :scroll: Code of Conduct

TBA.

<!-- License -->
## :warning: License

Distributed under the AGPL-3.0-or-later License. See [LICENSE](./LICENSE) for more information.

<!-- Contact -->
## :handshake: Contact

Immanent Tech — <hello@immanent.tech>

Project Link: [https://github.com/immanent-tech/foragd](https://github.com/immanent-tech/foragd)

<!-- Acknowledgments -->
## :gem: Acknowledgements
