<p align="center">
  <a href="https://foragd.app">
    <img src="https://github.com/immanent-tech/foragd/raw/main/web/assets/play-feature-image.webp" alt="Foragd hero image" width="1024" height="500">
  </a>
</p>

<h1 align="center">Foragd</h1>

<p align="center">
  A beautiful, web based, online feed reader.
  <br />
  Keep your RSS, Atom, and other syndication sources in one place — stay up to date across mobile, tablet, desktop, and laptop.
  <br />
  <br />
  <a href="https://foragd.app"><strong>foragd.app »</strong></a>
</p>

<p align="center">
  <a href="https://github.com/immanent-tech/foragd/graphs/contributors"><img src="https://img.shields.io/github/contributors/immanent-tech/foragd" alt="Contributors"></a>
  <a href="https://github.com/immanent-tech/foragd/blob/main"><img src="https://img.shields.io/github/last-commit/immanent-tech/foragd" alt="Last update"></a>
  <a href="https://github.com/immanent-tech/foragd/network/members"><img src="https://img.shields.io/github/forks/immanent-tech/foragd" alt="Forks"></a>
  <a href="https://github.com/immanent-tech/foragd/stargazers"><img src="https://img.shields.io/github/stars/immanent-tech/foragd" alt="Stars"></a>
  <a href="https://github.com/immanent-tech/foragd/issues"><img src="https://img.shields.io/github/issues/immanent-tech/foragd" alt="Open issues"></a>
  <a href="https://github.com/immanent-tech/foragd/blob/main/LICENSE"><img src="https://img.shields.io/github/license/immanent-tech/foragd" alt="License"></a>
</p>

---

## 📔 Table of Contents

- [📔 Table of Contents](#-table-of-contents)
- [🌟 About the Project](#-about-the-project)
  - [What Makes Foragd Different?](#what-makes-foragd-different)
  - [📷 Screenshots](#-screenshots)
  - [👾 Tech Stack](#-tech-stack)
  - [🎯 Features](#-features)
- [🧰 Getting Started](#-getting-started)
  - [‼️ Prerequisites](#️-prerequisites)
  - [⚙️ Installation](#️-installation)
  - [🧪 Running Tests](#-running-tests)
  - [🏃 Run Locally](#-run-locally)
  - [🚩 Deployment](#-deployment)
- [👀 Usage](#-usage)
- [👋 Contributing](#-contributing)
  - [📜 Code of Conduct](#-code-of-conduct)
- [⚠️ License](#️-license)
- [🤝 Contact](#-contact)
- [💎 Acknowledgements](#-acknowledgements)

## 🌟 About the Project

Foragd is an online, web-based feed reader for all syndication formats (RSS, Atom, JSONFeed).

### What Makes Foragd Different?

- **Focused on reading content, not tracking totals.** No shoehorning feeds into an email-like inbox, no unread-count badges to chase. Just a straightforward homepage, subscription view, and article view.
- **Powerful search.** Fast, full-text search across subscriptions and articles — find that one article, from that one site, about that one thing, without digging.
- **Easy filtering.** Filter articles within a subscription by keyword, phrase, category, or author, using simple `+/-` operators — no complex filter builder required. For example:
  `alcoholic drinks + -"rum based" +daiquiri`

### 📷 Screenshots

<p align="center">
  <img src="https://github.com/immanent-tech/foragd/raw/main/web/content/screenshots/main.webp" alt="Foragd home page on desktop and mobile" width="800">
</p>

### 👾 Tech Stack

**Server**

- [Go](https://go.dev/)
- [Chi](https://go-chi.io/)
- [templ](https://templ.guide/)
- [htmx](https://htmx.org/)
- [Tailwind CSS](https://tailwindcss.com/)
- [Daisy UI](https://daisyui.com/)

**Data store**

- [Elasticsearch](https://www.elastic.co/)

**Backend services**

- [Auth0](https://auth0.com/) — authentication
- [Stripe](https://stripe.com/) — billing
- [Resend](https://resend.com/) — transactional email

### 🎯 Features

- **Mobile and desktop friendly.** Foragd is a web based app that works in any modern browser, on any device.
- **Article filtering.** Filter articles in a subscription by text/phrase, category, or author with simple, powerful operators.
- **Content comes first.** A simple UI, big images, and clean typography let the content shine.
- **Powerful search.** Full-text search across subscriptions and articles, with quick access and actions right from the search bar.
- **Subscription customisation.** Give subscriptions a nickname and organise them with your own categories.
- **Favorites.** Mark subscriptions and articles as favorites for quick access later.
- **Grouped subscriptions.** Combine multiple subscriptions into a single unified view — great for following several sources on the same topic.
- **Search subscriptions.** Save a search as a subscription so new matching articles keep showing up automatically. Perfect for tracking news or topics across many sources at once.

## 🧰 Getting Started

### ‼️ Prerequisites

To run Foragd, you'll need:

- [Podman](https://podman.io/) or [Docker](https://www.docker.com/)
- An [Elasticsearch](https://www.elastic.co/) instance
- An [Auth0](https://auth0.com/) account
- A [Stripe](https://stripe.com/) account
- A [Resend](https://resend.com/) account
- A GCP project (for supporting cloud services)

### ⚙️ Installation

TBA.

### 🧪 Running Tests

TBA.

### 🏃 Run Locally

TBA.

### 🚩 Deployment

TBA.

## 👀 Usage

Foragd is available today at **[foragd.app](https://foragd.app)**. Sign up, add your first feeds, and start reading.

## 👋 Contributing

<a href="https://github.com/immanent-tech/foragd/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=immanent-tech/foragd" alt="Contributors" />
</a>

Contributions are always welcome!

See `contributing.md` for ways to get started.

### 📜 Code of Conduct

TBA.

## ⚠️ License

Distributed under the AGPL-3.0-or-later License. See [LICENSE](https://github.com/immanent-tech/foragd/blob/main/LICENSE) for more information.

## 🤝 Contact

Immanent Tech — [hello@immanent.tech](mailto:hello@immanent.tech)

Project Link: [github.com/immanent-tech/foragd](https://github.com/immanent-tech/foragd)

## 💎 Acknowledgements

TBA.
