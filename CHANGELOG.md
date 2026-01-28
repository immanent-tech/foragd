# Changelog

## [0.36.0](https://github.com/immanent-tech/foragd/compare/v0.35.0...v0.36.0) (2026-01-28)


### Features

* :sparkles: add posts (blog) ([310f94f](https://github.com/immanent-tech/foragd/commit/310f94fee6ddeebea5d39ada9335d67bcedc0e92))
* :sparkles: customise user-agent for fetching content and publish a doc about how content is fetched ([bb58af0](https://github.com/immanent-tech/foragd/commit/bb58af094e05ae139aa6a071d675be820de2bd67))
* **handlers:** :sparkles: internal errors can now be rendered in page or as notification depending on request method ([afc6136](https://github.com/immanent-tech/foragd/commit/afc6136abf68df96064ccecd1ac835691ce28a0a))


### Bug Fixes

* :bug: viewer is now made of self-contained templates ([001abbd](https://github.com/immanent-tech/foragd/commit/001abbde1f29b4ee9e390a2e51b7f41811ed3f4d))
* **handlers:** :bug: oob update dock/sidebar on partial response for list subscriptions ([947faaf](https://github.com/immanent-tech/foragd/commit/947faaf60c3b548098e9db2ff1ac40ba0252aa3b))
* **scheduler:** :wrench: fix user-agent string ([0f78dfa](https://github.com/immanent-tech/foragd/commit/0f78dfa5121ebdf3933f25c2ab1c565d69ec0819))
* **templates:** :bug: ensure unique templ fragment keys ([4dfb4a3](https://github.com/immanent-tech/foragd/commit/4dfb4a34f6c61268837e6733eb6bc4734b2e9c82))
* **templates:** :bug: fix footer positioning ([3d24375](https://github.com/immanent-tech/foragd/commit/3d2437532d9fc81f7af92c7b319b02b1d2931cea))
* **templates:** :lipstick: fix bottom margin on about page ([b04a77e](https://github.com/immanent-tech/foragd/commit/b04a77e9037e820559ab831a1e62236694f77e70))
* **templates:** :lipstick: make sure docs are centered on page ([b114196](https://github.com/immanent-tech/foragd/commit/b1141965524445eba05070cb333cadd44d71954d))

## [0.35.0](https://github.com/immanent-tech/foragd/compare/v0.34.1...v0.35.0) (2026-01-27)


### Features

* **schema:** :sparkles: add a case-insentive keyword sub-field on categories ([8ae8554](https://github.com/immanent-tech/foragd/commit/8ae8554bab2baa0b50680a678ce85edf849e75d7))
* **templates:** :sparkles: new simpler home page layout ([6075479](https://github.com/immanent-tech/foragd/commit/6075479cc17d8c327befba2a3187e68bfc5982b7))


### Bug Fixes

* **auth0:** :bug: improve logic around refreshing tokens ([d5fb968](https://github.com/immanent-tech/foragd/commit/d5fb968db6b45f730c3ca2ac21478824d1e0305f))

## [0.34.1](https://github.com/immanent-tech/foragd/compare/v0.34.0...v0.34.1) (2026-01-23)


### Bug Fixes

* :bug: fix display of modals ([bb90a17](https://github.com/immanent-tech/foragd/commit/bb90a17efbc5ac49bcc5be5a1b73ad6696143156))

## [0.34.0](https://github.com/immanent-tech/foragd/compare/v0.33.0...v0.34.0) (2026-01-22)


### Features

* :sparkles: add ability to change the max view history of articles ([d1be722](https://github.com/immanent-tech/foragd/commit/d1be722861178ee60d49186bff6ce63d7d268987))


### Bug Fixes

* **handlers:** :bug: properly handle nothing unread on homepage ([bd27e05](https://github.com/immanent-tech/foragd/commit/bd27e05bde0e8587021c5ba913baa8a369bc169c))
* **templates:** :bug: display fixes when there is nothing to show (no subscriptions and/or articles) ([7b19c3f](https://github.com/immanent-tech/foragd/commit/7b19c3fee6ffaa67dd60016a9c5b9e36fd873309))


### Performance Improvements

* **handlers:** :zap: watch for updates improvements ([7cf7763](https://github.com/immanent-tech/foragd/commit/7cf7763d22dfc574f9630acdd595c07365f21ac8))

## [0.33.0](https://github.com/immanent-tech/foragd/compare/v0.32.0...v0.33.0) (2026-01-22)


### Features

* **models:** :sparkles: add a semantic text sub-field to feed description ([2389a5b](https://github.com/immanent-tech/foragd/commit/2389a5b25ff0a95650aba7109391d799cd40fc92))


### Bug Fixes

* **scheduler:** :bug: ignore "new" feeds that already have a scheduled job (it just hasn't run or ran with errors) ([a2bfde4](https://github.com/immanent-tech/foragd/commit/a2bfde4b475f70fd93edb0ca3e8f4c6d1c7ae144))
* **templates:** :bug: simplify logic for hx-swap for auto-marking article as read ([dfa9848](https://github.com/immanent-tech/foragd/commit/dfa984851daa05af9087e9bb8ad709a19a1381bf))
* **templates:** :bug: view article fixes ([00e2533](https://github.com/immanent-tech/foragd/commit/00e25337aac78f164134e9b9a4a824f53eb82bc4))

## [0.32.0](https://github.com/immanent-tech/foragd/compare/v0.31.1...v0.32.0) (2026-01-21)


### Features

* :sparkles: policy docs served through metadata ([9699f24](https://github.com/immanent-tech/foragd/commit/9699f24204bfe9016eed5ee477b2d8c8c11717c7))
* **templates:** :sparkles: improved styling specifically for email newsletters ([a12b650](https://github.com/immanent-tech/foragd/commit/a12b6504c3449f92faafc55f54512325a331dc3c))
* **templates:** :sparkles: update subscriptions limit in pricing ([6c9f93d](https://github.com/immanent-tech/foragd/commit/6c9f93db2b821a233600cab1d4dfeb5ef4809efe))


### Bug Fixes

* **auth0:** :bug: fix header format ([b5e0a09](https://github.com/immanent-tech/foragd/commit/b5e0a095cfc76ef2ae503499870ae68535168065))
* **templates:** :bug: show unknown as last updated value if feed doesn't report an updated/published value itself ([2089ab7](https://github.com/immanent-tech/foragd/commit/2089ab75d9bc1c401247c0ad868bf89e2a9a3506))

## [0.31.1](https://github.com/immanent-tech/foragd/compare/v0.31.0...v0.31.1) (2026-01-20)


### Bug Fixes

* **templates:** :bug: only show mark action when view is not all ([96e4219](https://github.com/immanent-tech/foragd/commit/96e4219fb418698ee3df45fb74f22d721335946d))
* **templates:** :bug: show a message if there are no categories to filter on ([ccff576](https://github.com/immanent-tech/foragd/commit/ccff57643a7e497fca7e72b3e76c95ef5e994d50))


### Performance Improvements

* **server:** :zap: simpler graceful shutdown logic ([03685d3](https://github.com/immanent-tech/foragd/commit/03685d3cd19da7a05ac4fa1375a532e8ef94b84c))

## [0.31.0](https://github.com/immanent-tech/foragd/compare/v0.30.0...v0.31.0) (2026-01-20)


### Features

* :sparkles: add mark and favorite buttons on article cards and out of menu ([ec1f889](https://github.com/immanent-tech/foragd/commit/ec1f889c8e9cc93da3263a2d90e0e43b456602df))
* :sparkles: add mark and favorite buttons on subscription cards and out of menu ([4a4a29c](https://github.com/immanent-tech/foragd/commit/4a4a29cb0ff80c07cb7a684c9116fe1dee0bcce3))


### Bug Fixes

* **templates:** :bug: mark all subscription articles works again ([f4de1cb](https://github.com/immanent-tech/foragd/commit/f4de1cb684600128b836ca79f3c20933b7757942))

## [0.30.0](https://github.com/immanent-tech/foragd/compare/v0.29.1...v0.30.0) (2026-01-19)


### Features

* **logging:** :sparkles: add a replacer to format logs with gcp logging format in containers ([5c29d4b](https://github.com/immanent-tech/foragd/commit/5c29d4b831c2fec6008912840a82d4b1250b7970))


### Bug Fixes

* **scheduler:** :bug: ensure single job context is used ([3506f8e](https://github.com/immanent-tech/foragd/commit/3506f8ebc6fc917ec9bde934e97742a5e86987d4))

## [0.29.1](https://github.com/immanent-tech/foragd/compare/v0.29.0...v0.29.1) (2026-01-19)


### Bug Fixes

* **models:** :bug: add missing return ([3da4bb8](https://github.com/immanent-tech/foragd/commit/3da4bb8de9bdf9dba3ea40a22289a72c63ad19e2))

## [0.29.0](https://github.com/immanent-tech/foragd/compare/v0.28.1...v0.29.0) (2026-01-19)


### Features

* **auth0:** :sparkles: use refresh tokens to keep a user logged in ([af22864](https://github.com/immanent-tech/foragd/commit/af22864f8e0fb23bbedea2a5b5728e75ff43b3cd))


### Bug Fixes

* **models:** :bug: if feed details cannot be retrieved, set a default update interval ([dfc5aa1](https://github.com/immanent-tech/foragd/commit/dfc5aa1cc604464c79548231d8fcb1bced16727d))
* **templates:** :bug: ensure all required parameters are passed to mark article as read after viewing ([c10cfb6](https://github.com/immanent-tech/foragd/commit/c10cfb6d65ece3f0dc9edf025ede26d06f111cde))

## [0.28.1](https://github.com/immanent-tech/foragd/compare/v0.28.0...v0.28.1) (2026-01-16)


### Bug Fixes

* :bug: fix passing filters when paginating results ([592e520](https://github.com/immanent-tech/foragd/commit/592e5205d562cf51dbda44661056318ec0ff84c3))
* **search:** :bug: also match phrase prefix when searching for results ([e09956e](https://github.com/immanent-tech/foragd/commit/e09956e76220eaec57fdc92d5c56d8d1b3f389d0))
* **search:** :bug: fix search pagination ([b46f580](https://github.com/immanent-tech/foragd/commit/b46f5802df7fd352d1392a9da7d6ad1f788e5127))

## [0.28.0](https://github.com/immanent-tech/foragd/compare/v0.27.0...v0.28.0) (2026-01-16)


### Features

* **templates:** :sparkles: viewer can now extract images from feed content if no image was supplied in item object ([6341bcd](https://github.com/immanent-tech/foragd/commit/6341bcd94440bac494d7417b1fd39510e72a4a43))


### Bug Fixes

* **models:** :bug: fix filter articles logic ([3c18376](https://github.com/immanent-tech/foragd/commit/3c18376a258406ede3c77b937c6a5542310f99fe))

## [0.27.0](https://github.com/immanent-tech/foragd/compare/v0.26.0...v0.27.0) (2026-01-16)


### Features

* :sparkles: dynamic list of category filters based on shown subscriptions/articles ([e6bb133](https://github.com/immanent-tech/foragd/commit/e6bb1334ad01d59b1d571b93854472aab6178376))
* **cli:** :sparkles: improve cli user management ([d96d1d8](https://github.com/immanent-tech/foragd/commit/d96d1d81664029a902dc88d19fd0121144ecd969))
* **templates:** :lipstick: improved notification styling ([f5159f4](https://github.com/immanent-tech/foragd/commit/f5159f46143f92408d8ac38bc409a27fc615fa2f))
* **templates:** :lipstick: visually improved actions on article content ([8f3a0bc](https://github.com/immanent-tech/foragd/commit/8f3a0bcc0cd279acde656cd6ffcbd49c1cc05047))


### Bug Fixes

* :bug: fix list of category filters for subscriptions ([70faad0](https://github.com/immanent-tech/foragd/commit/70faad057cd0131aa683365dc9fa7e145fab1a89))
* :bug: mark all subscriptions now handles marking all subscriptions after pagination ([bafc7cd](https://github.com/immanent-tech/foragd/commit/bafc7cdd94c44265e6e096080e0bdea439f492ba))
* **templates:** :bug: improved category filter listing ([bbc7fe0](https://github.com/immanent-tech/foragd/commit/bbc7fe085f4218d8782f10daff7913e80a06e14f))
* **templates:** :bug: move subscription email so it shows up for accounts using a social login ([e4795ea](https://github.com/immanent-tech/foragd/commit/e4795ea2070f803971201ab0809c221f6dda5b08))

## [0.26.0](https://github.com/immanent-tech/foragd/compare/v0.25.0...v0.26.0) (2026-01-14)


### Features

* :sparkles: add frontend support for email subscriptions ([39abe1a](https://github.com/immanent-tech/foragd/commit/39abe1a2c3ec14ca427d7a40011244f533400f18))
* :sparkles: new plan/pricing ([182cc73](https://github.com/immanent-tech/foragd/commit/182cc735ca8fefa7ef72777261a04a9a0b563bf5))
* **models:** :sparkles: try to extract an image from the article content if one is not explicitly set ([08f8124](https://github.com/immanent-tech/foragd/commit/08f8124b36d2c29c9f98f0384891b8df85db0efc))


### Bug Fixes

* :bug: actually we do need to filter by user... ([3c994a4](https://github.com/immanent-tech/foragd/commit/3c994a429b7ffd32cc36d65ae682f07822608a04))
* :bug: don't generation subscription query clauses for subscription that aren't based on a feed source ([2adf4e8](https://github.com/immanent-tech/foragd/commit/2adf4e85fd0fd911fbc9844fc2a64f609c0a894c))
* **models:** :bug: fix article generation from items ([b37967d](https://github.com/immanent-tech/foragd/commit/b37967dae792eca979717d06095bd26398a871ee))
* **templates:** :bug: remove extraneous text on viewer page ([e4cfc1e](https://github.com/immanent-tech/foragd/commit/e4cfc1e5a04ac1c031e60eda19a1280fba040878))

## [0.25.0](https://github.com/immanent-tech/foragd/compare/v0.24.0...v0.25.0) (2026-01-13)


### Features

* :sparkles: add support for email subscriptions ([75778bc](https://github.com/immanent-tech/foragd/commit/75778bcd566b8daac6a230d9e9278625f24c30d3))


### Bug Fixes

* **templates:** :bug: remove modal in wrong place ([91095bb](https://github.com/immanent-tech/foragd/commit/91095bbf53dd9729f247b9e84c992df65f7c0447))
* **templates:** :bug: remove modal in wrong place ([4e4d166](https://github.com/immanent-tech/foragd/commit/4e4d1662871b80657a0313bf97112270ffc5cc15))

## [0.24.0](https://github.com/immanent-tech/foragd/compare/v0.23.1...v0.24.0) (2026-01-12)


### Features

* :sparkles: when creating update feed jobs, support setting interval to value derived from feed details, if set ([56e2518](https://github.com/immanent-tech/foragd/commit/56e25189199db50fea7ab8fff92b8bb064a66cce))
* **models:** :sparkles: if a feed isn't strictly valid (according to the format spec), just use it anyway ([1c86e73](https://github.com/immanent-tech/foragd/commit/1c86e7304f8ae524e5ddec5e913845bf57f422ee))
* **scheduler:** :sparkles: add a cli command for clearing the job queue ([1139dfe](https://github.com/immanent-tech/foragd/commit/1139dfe1f4a268ef1ffc50d03caddef2f0de1638))
* **scheduler:** :sparkles: add a cli command for initialising the scheduler/job queue ([9f25731](https://github.com/immanent-tech/foragd/commit/9f25731cb752502301c95974ddd268a7493b4fff))


### Bug Fixes

* **cli:** :bug: when deleting a user, also delete their subscriptions ([2f1dc43](https://github.com/immanent-tech/foragd/commit/2f1dc4327ad8d7020c3d1d2012e3197e4ac6c735))
* **models:** :bug: get feed title from passed in feed details for items ([ddeb5df](https://github.com/immanent-tech/foragd/commit/ddeb5dfb0bd033c87fb44f410a4e85ea22a102e3))
* **scheduler:** :bug: also clear job states when clearing job queue ([46210ad](https://github.com/immanent-tech/foragd/commit/46210adb259c6f8576a371f39bbbd0800cb76fde))
* **scheduler:** :bug: handle int64 as duration when parsing poll job triggers ([b4e6324](https://github.com/immanent-tech/foragd/commit/b4e6324ddad134b60c3b27439e009266f259f6a3))

## [0.23.1](https://github.com/immanent-tech/foragd/compare/v0.23.0...v0.23.1) (2026-01-11)


### Bug Fixes

* **templates:** :lipstick: make sure subscription card indicates it is clickable with cursor ([5381b2d](https://github.com/immanent-tech/foragd/commit/5381b2dbcd81faac16edbd28b806f264a7393d36))


### Reverts

* change pricing to TBD until its finalised ([0130a03](https://github.com/immanent-tech/foragd/commit/0130a03d824a36e2c136727f5f678dda6a813337))

## [0.23.0](https://github.com/immanent-tech/foragd/compare/v0.22.1...v0.23.0) (2026-01-11)


### Features

* **models:** :sparkles: start supporting finding feeds for specific sites with APIs/quirks ([c9c19a6](https://github.com/immanent-tech/foragd/commit/c9c19a6c3d26c7c745f3930dba0e3fac1c605b22))


### Bug Fixes

* **templates:** :bug: fix context menu sizing on subscription cards ([c00e105](https://github.com/immanent-tech/foragd/commit/c00e1055fe1973be5ff40bac39a41468aba32671))

## [0.22.1](https://github.com/immanent-tech/foragd/compare/v0.22.0...v0.22.1) (2026-01-11)


### Performance Improvements

* **templates:** :zap: add lazy and async image loading on feed viewer ([9a3780a](https://github.com/immanent-tech/foragd/commit/9a3780a5ef6e9a0f252a3c1cdf012ca97fb0de4c))

## [0.22.0](https://github.com/immanent-tech/foragd/compare/v0.21.0...v0.22.0) (2026-01-10)


### Features

* :sparkles: allow docs access externally ([0774280](https://github.com/immanent-tech/foragd/commit/0774280d7dd3a0528eda9c9be0701f65d1b56ee1))


### Bug Fixes

* **templates:** :bug: add missing share model when viewing article content ([8750007](https://github.com/immanent-tech/foragd/commit/87500071e538b5c1e50bfa4e5adad0ad02d0d723))

## [0.21.0](https://github.com/immanent-tech/foragd/compare/v0.20.0...v0.21.0) (2026-01-10)


### Features

* :sparkles: add an about page ([1f7cfd6](https://github.com/immanent-tech/foragd/commit/1f7cfd647ac15ee63f13234a1c5afc43f5ce6284))
* **config:** :sparkles: better default app description ([3079103](https://github.com/immanent-tech/foragd/commit/3079103f6c83dacace6389adf5792db5f1fc9887))
* **templates:** :recycle: reworked template rendering ([2b88e48](https://github.com/immanent-tech/foragd/commit/2b88e48c8353a5632e0796a38861372537bc936f))
* **templates:** :sparkles: opengraph properties support ([4e4a640](https://github.com/immanent-tech/foragd/commit/4e4a64042902b83ac31cb370a60acaa58c6bc84a))


### Bug Fixes

* **templates:** :bug: don't use htmx for home link in header ([cfbea2c](https://github.com/immanent-tech/foragd/commit/cfbea2c0851b36448ee1f73f96ccddf358f0703f))
* **templates:** :bug: fix footer links ([814529d](https://github.com/immanent-tech/foragd/commit/814529d9ee454df9dbdf722553700797f9718537))
* **templates:** :bug: fix link to pricing ([6e4db72](https://github.com/immanent-tech/foragd/commit/6e4db7233e3601be4a8f224ce2a6f0156fe9d578))

## [0.20.0](https://github.com/immanent-tech/foragd/compare/v0.19.0...v0.20.0) (2026-01-10)


### Features

* **assets:** :sparkles: add htmx-head-support htmx extension ([193d78d](https://github.com/immanent-tech/foragd/commit/193d78d73483939943784b840602d09d89422744))
* **templates:** :sparkles: add header and footer to document pages ([b7e0557](https://github.com/immanent-tech/foragd/commit/b7e055780640daec6a311e817ec7bd8043250f4b))
* **templates:** :sparkles: add header and footer to inspector page ([4d40512](https://github.com/immanent-tech/foragd/commit/4d40512347c4750afc1fc03ca56b52657d638ac9))
* **templates:** :sparkles: flexible and reusable header and footer ([2bd66b4](https://github.com/immanent-tech/foragd/commit/2bd66b40d06b62e627ead9568471deb1a9dc49dc))
* **templates:** :sparkles: use card layout for inspector results ([f3e93b2](https://github.com/immanent-tech/foragd/commit/f3e93b2659fee27ae1648e236d7eb5a38b99b883))


### Bug Fixes

* **templates:** :bug: also enlarge article images if required ([7da66ef](https://github.com/immanent-tech/foragd/commit/7da66ef968587ba13a06d091f903264a61aa6182))
* **templates:** :bug: handle article images that don't match our desired aspect ratio ([dd3ba37](https://github.com/immanent-tech/foragd/commit/dd3ba37beea1103fcab8c150070e13d9935104c9))
* **templates:** :bug: make sure article images are full width on cards ([61f6186](https://github.com/immanent-tech/foragd/commit/61f61869ef39e8bcf998249a29d28054ad111881))
* **templates:** :bug: make sure subscription thumbnail can be generated ([05ceeea](https://github.com/immanent-tech/foragd/commit/05ceeea98f57e0a1874b0ebc89de4f055526f0db))

## [0.19.0](https://github.com/immanent-tech/foragd/compare/v0.18.0...v0.19.0) (2026-01-09)


### Features

* :sparkles: add a feed inspector to inspect feeds on websites ([8721f8c](https://github.com/immanent-tech/foragd/commit/8721f8c288a5729684b1369313065a7a7cf8abdf))

## [0.18.0](https://github.com/immanent-tech/foragd/compare/v0.17.0...v0.18.0) (2026-01-08)


### Features

* **templates:** :sparkles: top-left logo is a link back to homepage ([185b515](https://github.com/immanent-tech/foragd/commit/185b51502d9f5429bd4b6d56e87ce94052c392f8))


### Bug Fixes

* **handlers:** :bug: show "all caught up" if no unread on homepage ([54a8129](https://github.com/immanent-tech/foragd/commit/54a812998a5c7a478181baf2e1acebcf9b8d54ef))

## [0.17.0](https://github.com/immanent-tech/foragd/compare/v0.16.0...v0.17.0) (2026-01-08)


### Features

* :memo: add license and update readme ([b6e2b0f](https://github.com/immanent-tech/foragd/commit/b6e2b0fd92b2748cac5961b56d46d9c13625524b))


### Bug Fixes

* :memo: remove extra element ([ea2ee4f](https://github.com/immanent-tech/foragd/commit/ea2ee4f5ceadcd7c058fb92ea6105abaaee7a7be))

## [0.16.0](https://github.com/immanent-tech/foragd/compare/v0.15.0...v0.16.0) (2026-01-08)


### Features

* **assets:** :sparkles: add techzine.eu to informed feedset ([432b239](https://github.com/immanent-tech/foragd/commit/432b239554702af4dc80c0ff49692c0c72a26d31))


### Bug Fixes

* :bug: fix filtering new subscriptions required ([52ac582](https://github.com/immanent-tech/foragd/commit/52ac582c3c413b1acfe39c7100766be1e4ce0aa9))
* **handlers:** :bug: fetch all user feed subscriptions when exporting ([1fa16d3](https://github.com/immanent-tech/foragd/commit/1fa16d39c45491c92dea374434342650bf7e088f))
* **models:** :bug: improved new feed subscription handling ([60688a4](https://github.com/immanent-tech/foragd/commit/60688a40b3c34478c621444a8250031698c0dae7))
* **templates:** :bug: make sure csrf token is included in plan selection requests ([bed1ef9](https://github.com/immanent-tech/foragd/commit/bed1ef99f5c35dead9f098ca4fd38234497901f7))
* **templates:** :bug: remove inline js browser timezone fetching ([9aed599](https://github.com/immanent-tech/foragd/commit/9aed5993e63facd20f91654e6411301ccbd436a9))

## [0.15.0](https://github.com/immanent-tech/foragd/compare/v0.14.0...v0.15.0) (2026-01-07)


### Features

* **github:** :wrench: support configuring issues repo through envrionment variables ([61cd9b8](https://github.com/immanent-tech/foragd/commit/61cd9b8e13592f07835a4b39d0f31aee1137aff4))
* **resend:** :sparkles: add resend integration ([4b638a0](https://github.com/immanent-tech/foragd/commit/4b638a07251446847b86033be2b7f0dd11265164))


### Bug Fixes

* **resend:** :bug: ignore csrf checks for resend webhooks ([98fdb5a](https://github.com/immanent-tech/foragd/commit/98fdb5a1308baa4ce18736875f744a256b843197))

## [0.14.0](https://github.com/immanent-tech/foragd/compare/v0.13.0...v0.14.0) (2026-01-07)


### Features

* **assets:** :lipstick: nicer notification transitions ([4e311bc](https://github.com/immanent-tech/foragd/commit/4e311bc129fc6705c13989d302dcba9c3f8589da))
* **handlers:** :sparkles: add handler for generic auth0 backend errors ([391f1f7](https://github.com/immanent-tech/foragd/commit/391f1f77bb4a635850ebd184378ba30b3d9713c9))
* **handlers:** :zap: use server push to load critical assets before being requested ([54392b9](https://github.com/immanent-tech/foragd/commit/54392b99690a7d297beec736c3cf12ed23b92d4d))
* **server:** :zap: add a write timeout ([b7a920a](https://github.com/immanent-tech/foragd/commit/b7a920aa611d1e03bfc3cb305807fb22f5659168))
* **templates:** :sparkles: implement a common base view struct ([c8dbbe0](https://github.com/immanent-tech/foragd/commit/c8dbbe0ce2fde2eb79094026e391872913d89a28))


### Bug Fixes

* **handlers:** :bug: fix error scope ([263ae83](https://github.com/immanent-tech/foragd/commit/263ae83c1c59448e8649ba072a6f44ffa8400466))
* **models:** :bug: fix logic for detecting urls without a scheme ([03ef589](https://github.com/immanent-tech/foragd/commit/03ef5891aa5337aac9aaac18ca514bd8f21247c1))

## [0.13.0](https://github.com/immanent-tech/foragd/compare/v0.12.0...v0.13.0) (2026-01-01)


### Features

* **server:** :sparkles: cleaner server startup and shutdown ([b4c3db5](https://github.com/immanent-tech/foragd/commit/b4c3db53cbe54c52a684c56a68c6a2ff5fddf3b2))
* **templates:** :sparkles: if the user updates their avatar, show the new avatar immediately ([c8c7290](https://github.com/immanent-tech/foragd/commit/c8c7290617d4f83e2200f5b31d0ad5fa727f426c))


### Bug Fixes

* **scheduler:** :bug: clean up scheduler shutdown ([8e80fca](https://github.com/immanent-tech/foragd/commit/8e80fcadafd90e9ee855142bd0ca43920951f79b))
* **templates:** :bug: add missing top margin to header on home page ([d4d6563](https://github.com/immanent-tech/foragd/commit/d4d6563e4a654662b98253144b7c2376a59711dc))
* **templates:** :bug: align buttons on unsubscribe modal ([a2b3887](https://github.com/immanent-tech/foragd/commit/a2b388749a90c1031dd3c98c17c69510c6e9d7af))
* **templates:** :bug: correct clicking subscription link when viewing article content ([afdabb1](https://github.com/immanent-tech/foragd/commit/afdabb16f77c0e764d88b220bf76e6e8de64a1fd))

## [0.12.0](https://github.com/immanent-tech/foragd/compare/v0.11.1...v0.12.0) (2026-01-01)


### Features

* **scheduler:** :sparkles: add job to clear deleted feeds from scheduler queue ([4757d75](https://github.com/immanent-tech/foragd/commit/4757d75d2bb20c2a4444ef516cf0e39e9e487db6))
* **scheduler:** :sparkles: update feed job improvements ([bf7420b](https://github.com/immanent-tech/foragd/commit/bf7420bab6bf34112f93419f97696022dbeb944f))
* **scheduler:** :sparkles: use the logger from context for the scheduler logger ([c720e19](https://github.com/immanent-tech/foragd/commit/c720e198467d6cf30762269c5725b9054af8b3ba))


### Bug Fixes

* **scheduler:** :bug: make sure fetching next scheduled job actually finds next scheduled job ([20f6a7e](https://github.com/immanent-tech/foragd/commit/20f6a7edcdc1d8db7e0741ef7d2d27e9a7280c0b))

## [0.11.1](https://github.com/immanent-tech/foragd/compare/v0.11.0...v0.11.1) (2025-12-31)


### Bug Fixes

* **assets:** :bug: make sure htmx global variable gets set correctly ([4b02fc6](https://github.com/immanent-tech/foragd/commit/4b02fc6b213c53f423dc17ab1a6ca7141f42d3d0))
* **assets:** :fire: remove debugging ([a4dc9d6](https://github.com/immanent-tech/foragd/commit/a4dc9d61e3aa6be686d362ad451559c1e7e555af))
* **config:** :bug: correct naming of environments ([3d80c93](https://github.com/immanent-tech/foragd/commit/3d80c935f7e14732d35eeba8c676e8b6c9c142ed))

## [0.11.0](https://github.com/immanent-tech/foragd/compare/v0.10.0...v0.11.0) (2025-12-30)


### Features

* **elastic:** :sparkles: add default auto fuzziness value for match and multi_match queries ([0250f2e](https://github.com/immanent-tech/foragd/commit/0250f2ed6ce4691af2997710f03a741f4fa597e6))

## [0.10.0](https://github.com/immanent-tech/foragd/compare/v0.9.0...v0.10.0) (2025-12-30)


### Features

* :lock: use a nonces for CSP/templ/htmx ([9c53c58](https://github.com/immanent-tech/foragd/commit/9c53c58cd4647bc6475dc68781ace3b1d7ef70d7))
* :sparkles: allow uploading a screenshot when reporting issues ([bbd338e](https://github.com/immanent-tech/foragd/commit/bbd338e84ff09a9fc3154d3a335bfd346a76cc5e))
* **middlewares:** :sparkles: add ability to set script-src-attr CSP ([bf8acca](https://github.com/immanent-tech/foragd/commit/bf8accad29176f24ba4c9cae491b2b6ba40f738d))
* **templates:** :lipstick: default to using view transitions API for htmx swaps ([35253a8](https://github.com/immanent-tech/foragd/commit/35253a8eb7dadf53e39048fabed9c346b13306e5))


### Bug Fixes

* :bug: ensure consistent parameter name for uploaded thumbnail data is used ([4df0075](https://github.com/immanent-tech/foragd/commit/4df00753dde3082f11de21cca0518b32479e1b0c))
* **templates:** :bug: pass subscription id as appropriate to filters on list articles page ([bb99ada](https://github.com/immanent-tech/foragd/commit/bb99ada7cb07edddb08c1f463723f7b3bab6da17))
* **templates:** :lipstick: fix color of loading element ([76c6aa2](https://github.com/immanent-tech/foragd/commit/76c6aa2050343919d45b282819eb074d2a635d0c))
* **templates:** :lock: ensure csrf token is only updated once per render context ([ddc1cae](https://github.com/immanent-tech/foragd/commit/ddc1cae389d45166ebbf98756eeb7a4c5928010a))

## [0.9.0](https://github.com/immanent-tech/foragd/compare/v0.8.1...v0.9.0) (2025-12-29)


### Features

* :sparkles: allow user customisation of subscription thumbnail/image ([5986ace](https://github.com/immanent-tech/foragd/commit/5986ace03b0483b0ccd3578f74b6e0f4fdcb16d9))


### Bug Fixes

* :bug: fetch subscription details when listing articles for a single subscription and pass to template ([9b2d569](https://github.com/immanent-tech/foragd/commit/9b2d569f1134f2b9edb892e2764c7741a92a1ecc))
* **handlers:** :bug: make sure background jobs are started and cleaned-up properly ([d9967c4](https://github.com/immanent-tech/foragd/commit/d9967c4ce9f2d3f51f157dba1d493a074afafd6b))
* **handlers:** :bug: protect against potential nil pointer references ([463215b](https://github.com/immanent-tech/foragd/commit/463215b27064472ee9b619937f1284289f66ebe3))
* **templates:** :lipstick: fix article content styling ([03fb2c4](https://github.com/immanent-tech/foragd/commit/03fb2c475e9c79e4242da1032f5a50ebddda3e27))

## [0.8.1](https://github.com/immanent-tech/foragd/compare/v0.8.0...v0.8.1) (2025-12-28)


### Bug Fixes

* **templates:** :bug: correct back to top button location on tablet displays ([202785e](https://github.com/immanent-tech/foragd/commit/202785e98e5fff42cc355495d7a50ca7b880fd84))

## [0.8.0](https://github.com/immanent-tech/foragd/compare/v0.7.0...v0.8.0) (2025-12-27)


### Features

* **templates:** :lipstick: new homepage layout ([8855233](https://github.com/immanent-tech/foragd/commit/88552339e627fa9b24867037c56e76da25d90d23))


### Bug Fixes

* :bug: avoid nil pointer references when importing subscriptions ([b7adee5](https://github.com/immanent-tech/foragd/commit/b7adee574145483b635ed1a791be4b8ef0ad1fa6))
* :bug: return consistent results or suggestions for searches ([c6f9e04](https://github.com/immanent-tech/foragd/commit/c6f9e0493805defb29529767368efe58f8d1ef27))

## [0.7.0](https://github.com/immanent-tech/foragd/compare/v0.6.0...v0.7.0) (2025-12-26)


### Features

* **elastic:** :sparkles: add query names for most query clauses for easier debugging of queries ([b9606d6](https://github.com/immanent-tech/foragd/commit/b9606d64c4a2cb350b4f9c4f340c49a3a3d1fa38))


### Bug Fixes

* **elastic:** :bug: pass search options correctly when executing paginated items search ([07d3869](https://github.com/immanent-tech/foragd/commit/07d3869ad195f58cbba169c70e36889a781a5b31))

## [0.6.0](https://github.com/immanent-tech/foragd/compare/v0.5.1...v0.6.0) (2025-12-24)


### Features

* **templates:** :lipstick: major layout improvements ([2c7b2d5](https://github.com/immanent-tech/foragd/commit/2c7b2d520647e8d809a3021b9af5b1cf5b84ce4d))
* **templates:** :sparkles: add find similar articles action to context menu of article view ([69e327c](https://github.com/immanent-tech/foragd/commit/69e327c12e47af42a6ff9bd57366130752d1b7e2))

## [0.5.1](https://github.com/immanent-tech/foragd/compare/v0.5.0...v0.5.1) (2025-12-23)


### Bug Fixes

* **templates:** :bug: mention pricing is indicative during beta ([05eb33c](https://github.com/immanent-tech/foragd/commit/05eb33cb64c4cdf26c229063f4b41c892f6257ea))

## [0.5.0](https://github.com/immanent-tech/foragd/compare/v0.4.0...v0.5.0) (2025-12-22)


### Features

* **middlewares:** :sparkles: better CORS middleware ([c16b704](https://github.com/immanent-tech/foragd/commit/c16b70461f9f9fcffc6336de4e0a2b5177f63561))

## [0.4.0](https://github.com/immanent-tech/foragd/compare/v0.3.2...v0.4.0) (2025-12-22)


### Features

* **templates:** :sparkles: add a faq to the landing page ([c0db2dc](https://github.com/immanent-tech/foragd/commit/c0db2dc5b641a3073552ed19fb1956b3d90fa141))
* **templates:** :sparkles: add share link on view article page and other link refactoring ([35d4bcb](https://github.com/immanent-tech/foragd/commit/35d4bcb3662d7eddf463a91c2821d8214e223b01))


### Performance Improvements

* **handlers:** :zap: use a sync.Pool of bytes.Buffer for storing fetched article content ([ba122c9](https://github.com/immanent-tech/foragd/commit/ba122c9d28cc05433283cbabe4cace5b2298209b))

## [0.3.2](https://github.com/immanent-tech/foragd/compare/v0.3.1...v0.3.2) (2025-12-19)


### Bug Fixes

* **handlers:** :bug: fix missing response header when marking an article ([4497c28](https://github.com/immanent-tech/foragd/commit/4497c281e193b20866b9388c028c6e4f59c3a800))
* **handlers:** :bug: only use feed in title on listing articles when there are articles to show ([4be2dff](https://github.com/immanent-tech/foragd/commit/4be2dfffecb9cf5087f156878993f0f2d6ca5c71))
* **templates:** :bug: fix link to all subscription articles on article card ([6b4ee10](https://github.com/immanent-tech/foragd/commit/6b4ee10c41e4a5cdd14ae61f296ce5d3fb4070b4))


### Performance Improvements

* **handlers:** :zap: adjust caching of assets ([f86986c](https://github.com/immanent-tech/foragd/commit/f86986cb78fa71c52e38607de4b9ea95f3caea2d))
* **handlers:** :zap: always set a Cache-Control header on dynamic user content ([d2dc8b3](https://github.com/immanent-tech/foragd/commit/d2dc8b3566080afdf46ab27707d118c78fad87f3))
* **handlers:** :zap: force browser to re-validate to avoid outdated content ([b606dbe](https://github.com/immanent-tech/foragd/commit/b606dbeae7ba7eeb21a4e9eb7692afb910187a47))
* **handlers:** :zap: force browser to revalidate page on history restore requests ([e64c10a](https://github.com/immanent-tech/foragd/commit/e64c10af6c7bb786dd4100c397e3dfa1f9a3c7ee))

## [0.3.1](https://github.com/immanent-tech/foragd/compare/v0.3.0...v0.3.1) (2025-12-19)


### Bug Fixes

* **templates:** :bug: fix cloudflare asset caching issues ([d4ecf43](https://github.com/immanent-tech/foragd/commit/d4ecf43c26a08254bc5d3e3cb32ad1b56969b2e6))

## [0.3.0](https://github.com/immanent-tech/foragd/compare/v0.2.0...v0.3.0) (2025-12-19)


### Features

* **templates:** :sparkles: add opengraph meta tags ([0c1d25f](https://github.com/immanent-tech/foragd/commit/0c1d25f37668e428368c8a9611105e4ec253289c))


### Bug Fixes

* :lock: fix usage of Cross Origin security headers ([2c5c91c](https://github.com/immanent-tech/foragd/commit/2c5c91c82642b3885f6a9dcc71c660f95462f8ec))
* **templates:** :bug: fix feature bento layout issues on mobile ([ec61d34](https://github.com/immanent-tech/foragd/commit/ec61d3476ffe4fc89a9fdd3b6f1539391e4f0f37))
* **templates:** :bug: landing page fixes ([0d1cb37](https://github.com/immanent-tech/foragd/commit/0d1cb37fecec940b1f282988d269198e8bc6d56d))
* **templates:** :bug: remove personal use note on Curator plan pricing features list ([0aa7a3c](https://github.com/immanent-tech/foragd/commit/0aa7a3c8db0ab51aa6d18b3f1aac64003fc7308c))

## [0.2.0](https://github.com/immanent-tech/foragd/compare/v0.1.1...v0.2.0) (2025-12-19)


### Features

* **templates:** :lipstick: reworked landing page ([d5dded2](https://github.com/immanent-tech/foragd/commit/d5dded26305aca3b13d644a4a415a8f7f83a66ce))


### Bug Fixes

* **models:** :bug: simplify handling of page filter restoration on list pages ([ec59b8e](https://github.com/immanent-tech/foragd/commit/ec59b8e03774f78762266b47d461d3b3f929fccc))

## [0.1.1](https://github.com/immanent-tech/foragd/compare/v0.1.0...v0.1.1) (2025-12-18)


### Bug Fixes

* **handlers:** :bug: load correct cache in avatar handler ([c3f2fc9](https://github.com/immanent-tech/foragd/commit/c3f2fc945328eed7c73912e41e96df526d083171))

## [0.1.0](https://github.com/immanent-tech/foragd/compare/v0.0.4...v0.1.0) (2025-12-18)


### Features

* :sparkles: allow user to upload new avatar image ([bff8019](https://github.com/immanent-tech/foragd/commit/bff80195d1f898b9c2417a0242883317e95d9092))


### Bug Fixes

* **middlewares:** :lock: relax CORP on view article pages to allow remote image loading ([f14592b](https://github.com/immanent-tech/foragd/commit/f14592bf7702f2c97bc46f62e5e9efc4f38f1be6))
* **templates:** :bug: fix positioning of back to top button on tablet sized screens ([b019533](https://github.com/immanent-tech/foragd/commit/b019533e5075271abe7524cae8d7e9d544fbec5b))

## [0.0.4](https://github.com/immanent-tech/foragd/compare/v0.0.3...v0.0.4) (2025-12-17)


### Performance Improvements

* **handlers:** :zap: use a sync.Pool for image buffer ([1e5fe00](https://github.com/immanent-tech/foragd/commit/1e5fe00d3ee3e9504ea2f32c1464d05024b4f658))

## [0.0.3](https://github.com/immanent-tech/foragd/compare/v0.0.2...v0.0.3) (2025-12-16)


### Bug Fixes

* :bug: first login fixes ([d4f9319](https://github.com/immanent-tech/foragd/commit/d4f9319128dac3594bbbfcccb7ba3c072d3db7c1))
* **scheduler:** :bug: ensure values are added to context after proper creation ([02dd35c](https://github.com/immanent-tech/foragd/commit/02dd35c2b819987e0d5570609e876439874ed147))

## [0.0.2](https://github.com/immanent-tech/foragd/compare/v0.0.1...v0.0.2) (2025-12-15)


### Miscellaneous Chores

* release 0.0.2 ([620cf15](https://github.com/immanent-tech/foragd/commit/620cf15e91e689bb7e1981b0323f3a273c4581cb))

## 0.0.1 (2025-12-14)


### Features

* :lipstick: new default theme ([c761534](https://github.com/immanent-tech/foragd/commit/c761534ae4891fbc42a4b315cc96f0bdec5e25df))
* :memo: add help documentation ([b886226](https://github.com/immanent-tech/foragd/commit/b886226190a3d9e04abf8299ad55cf34307c6531))
* :recycle: better article marking ([c1fa667](https://github.com/immanent-tech/foragd/commit/c1fa66771cdc13d192f561ff3a790c1e9d609aec))
* :recycle: better subscription marking ([c5ba2e6](https://github.com/immanent-tech/foragd/commit/c5ba2e62312586f90146f03c48bc3cc39b0c1b02))
* :recycle: improved unsubscribe process ([c9444ee](https://github.com/immanent-tech/foragd/commit/c9444ee5c8193352b1edfe3fc83844662830ec5b))
* :recycle: tweak home page results ([c110cc0](https://github.com/immanent-tech/foragd/commit/c110cc040f1111265f9fd0f8a85dc3a643d75890))
* :sparkles: add a mark subscription action on list articles page for single-subscription list ([51252ed](https://github.com/immanent-tech/foragd/commit/51252ed2f8971b79dd0220b081f574e48fc1779a))
* :sparkles: add ability to create search subscriptions ([d66133c](https://github.com/immanent-tech/foragd/commit/d66133ceb02ebd9d3b032760852a2d432c9198ec))
* :sparkles: add ability to filter subscriptions by favorites ([fc40046](https://github.com/immanent-tech/foragd/commit/fc4004697d179bddfb03900c49c9fc752eda2662))
* :sparkles: add group subscription type ([b83b659](https://github.com/immanent-tech/foragd/commit/b83b6598c223fa8b922923d42660f2121cec7ee1))
* :sparkles: add link in settings to go to Stripe to manage subscription plan ([487dc19](https://github.com/immanent-tech/foragd/commit/487dc197964ff1da48c8181bf84053e7a67c32fc))
* :sparkles: add more themes and change default theme to "silk" ([4063f97](https://github.com/immanent-tech/foragd/commit/4063f974a16e8e40ae7937e717c42efe91129de1))
* :sparkles: add subscription suggestions ([76ee02c](https://github.com/immanent-tech/foragd/commit/76ee02c8f257125f58e24cd2f32300ded5dc69cf))
* :sparkles: allow search results sort options ([7efa95c](https://github.com/immanent-tech/foragd/commit/7efa95c3bc430d917e0f9df27aa55c9b74ef3e5e))
* :sparkles: better marking articles ([adc1094](https://github.com/immanent-tech/foragd/commit/adc1094abd47ed9eb786966f300ec3c26d56ffce))
* :sparkles: deleting account also cancels subscription in stripe ([151f8b2](https://github.com/immanent-tech/foragd/commit/151f8b22e7f31da192830f22b643161625eda7da))
* :sparkles: handle account payment issues on login ([79c3b79](https://github.com/immanent-tech/foragd/commit/79c3b7978228bd78870dfa8960e04e763475a2b0))
* :sparkles: improved import ([c754e26](https://github.com/immanent-tech/foragd/commit/c754e2654add675b5c4b220b19f9847bb027913c))
* :sparkles: multisearch and search subscription updates ([548b886](https://github.com/immanent-tech/foragd/commit/548b8869b6f876aa8a84c652bf43301693db6282))
* :sparkles: search request improvements ([b206f48](https://github.com/immanent-tech/foragd/commit/b206f4840612b1fd0c4a774986d50a95fe948533))
* :sparkles: show subscription level on account settings page ([ac671ad](https://github.com/immanent-tech/foragd/commit/ac671ad00e6e067ff77ea1fc6ff0355a8c199050))
* :sparkles: update and improve policy doc links ([ac948b2](https://github.com/immanent-tech/foragd/commit/ac948b2f7a3b705228753d84ef4dc8cdd9c7af9f))
* **assets:** :bento: add a robots.txt ([1251c61](https://github.com/immanent-tech/foragd/commit/1251c610ad2d258b168823d762ca5c29300d7ab0))
* **assets:** :sparkles: add logo color variants ([fdd55b2](https://github.com/immanent-tech/foragd/commit/fdd55b226bc0db2b375b387db4d51360452d1f90))
* **assets:** :sparkles: add more subscriptions to the curated feed lists ([2150326](https://github.com/immanent-tech/foragd/commit/215032624baf9c14d62c7469c77c143cf447d103))
* **cli:** :sparkles: create a migrate schema cli command ([ccab611](https://github.com/immanent-tech/foragd/commit/ccab6115e3b0d0c4259dc59960a21a5336aaee12))
* **config:** :sparkles: set a pkg exported variable for checking what environment the service is running in ([8c4f7d9](https://github.com/immanent-tech/foragd/commit/8c4f7d90d666a31b67948c2aebaba0b18eed371b))
* **css:** :sparkles: add catppuccin mocha and latte themes ([01d800b](https://github.com/immanent-tech/foragd/commit/01d800b02a8fb8d3c0633aa6210dc8d57d2d3642))
* **handlers:** :sparkles: implement HX-Location setting directly ([8adae76](https://github.com/immanent-tech/foragd/commit/8adae76c0143cd663fe80f2598fd3b20d239c4a2))
* **handlers:** :sparkles: implement image caching to gcs ([b9f59ce](https://github.com/immanent-tech/foragd/commit/b9f59ce914e896c2693bc799a685909c3c8f0c48))
* **handlers:** :sparkles: set Cache-Control header for static content ([caa90fb](https://github.com/immanent-tech/foragd/commit/caa90fb36db435d7eb81e195449dcec76fe5ee44))
* **handlers:** :zap: add cache-control header on list pages ([f54b8b9](https://github.com/immanent-tech/foragd/commit/f54b8b9278d27ee2022c92613ccaf65aa3ec3b81))
* **middlewares:** :sparkles: load CSP from environment variables ([d9a92c0](https://github.com/immanent-tech/foragd/commit/d9a92c0e87bc09e72c5d4e59412091c4b9d85916))
* **models:** :sparkles: add more account settings ([c0148d1](https://github.com/immanent-tech/foragd/commit/c0148d1a6a0dd2d1a443764b1fc64202b4d88d49))
* **models:** :sparkles: calculate unread count for search subscriptions ([ff007e9](https://github.com/immanent-tech/foragd/commit/ff007e9f1b3b3a434cc19eda3db2926159e16f41))
* **models:** :sparkles: change default theme to latte ([f6a77ed](https://github.com/immanent-tech/foragd/commit/f6a77ed5f75f0dc6b151e5d029d616a2aeb1fa54))
* **models:** :sparkles: implement better file upload extraction and use for OPML file uploads ([0a709c1](https://github.com/immanent-tech/foragd/commit/0a709c18a40ca1a3c98b6a2d7777fce303f08d49))
* **models:** :sparkles: sort favorite subscriptions before others ([95c4a95](https://github.com/immanent-tech/foragd/commit/95c4a952860319fe5f416ba8c468ad116f122c1b))
* **server:** :lock: add some additional headers as recommended by OWASP guide for enhancing security ([267256d](https://github.com/immanent-tech/foragd/commit/267256d07412d2122bfe99dbb3eb5afd171662b4))
* **server:** :sparkles: add a robots.txt handler ([27e3c1e](https://github.com/immanent-tech/foragd/commit/27e3c1e7dcd6dc556369da1bead5490ff99e9cc7))
* **server:** :sparkles: add ability to block signup and login pages by config ([2a4c03a](https://github.com/immanent-tech/foragd/commit/2a4c03ad804501eb532cf0bebf0d5307f871e80c))
* **server:** :sparkles: add compression middleware ([be76a07](https://github.com/immanent-tech/foragd/commit/be76a07473bff858606299e31e057bdf59fa4ff6))
* **stripe:** :sparkles: handle subscription cancellation/deletion events ([3ee6c42](https://github.com/immanent-tech/foragd/commit/3ee6c426a4a7bff185c08e08b05d62d1da6af969))
* **templates:** :bento: add a screenshot to the landing page ([fede2d4](https://github.com/immanent-tech/foragd/commit/fede2d4b1af89ab250e04dede2a63ede2a000e23))
* **templates:** :lipstick: add back to top button on home page ([9235827](https://github.com/immanent-tech/foragd/commit/9235827262efad9bdfc5bbe803c079a9d24ee54e))
* **templates:** :lipstick: greatly improved article grid layout ([4499c11](https://github.com/immanent-tech/foragd/commit/4499c1153a17c6ba7b6af329de42221ec6e06305))
* **templates:** :lipstick: use transition api and show top of page when clicking link in sidebar ([44f898e](https://github.com/immanent-tech/foragd/commit/44f898ef2e164e8b6867c4dedb0eb55b90ef7e98))
* **templates:** :recycle: add a beta indicator on landing page ([17fe158](https://github.com/immanent-tech/foragd/commit/17fe158c7ac24252d7d29695cdc16b03dafeb771))
* **templates:** :recycle: use a forms template for form sections ([546d5a8](https://github.com/immanent-tech/foragd/commit/546d5a88223df5ea98fa054ac1967009f2c62504))
* **templates:** :sparkles: add a meta description ([e907349](https://github.com/immanent-tech/foragd/commit/e907349aa2a59df7e59caf2edea44a73561efea9))
* **templates:** :sparkles: add alt+a global shortcut for activating the actions menu on list subscriptions/articles pages ([491b7ff](https://github.com/immanent-tech/foragd/commit/491b7ff5ef040fe8e620ab7c139f19578d3abbd6))
* **templates:** :sparkles: add FAQs and CTA section to landing page ([30167f7](https://github.com/immanent-tech/foragd/commit/30167f70d366066e66d251c4e752e047d14d63b5))
* **templates:** :sparkles: add mark actions on search results page ([82d7dc2](https://github.com/immanent-tech/foragd/commit/82d7dc20dd29391e61e667e9b0c64fca670f72ef))
* **templates:** :sparkles: add menu action to add search subscription on subscriptions list page ([a443254](https://github.com/immanent-tech/foragd/commit/a443254816d4f92507a45186bb8a2c25fede65be))
* **templates:** :sparkles: add more global shortcuts ([1e868a9](https://github.com/immanent-tech/foragd/commit/1e868a9a3072b7ecff7bf00f89eecc0284d85b02))
* **templates:** :sparkles: add pricing section to landing page ([386f0a6](https://github.com/immanent-tech/foragd/commit/386f0a6a4f8ea5b54e26ffae9df496e83be8e79c))
* **templates:** :sparkles: add scroll to top button ([9fef6fd](https://github.com/immanent-tech/foragd/commit/9fef6fd2337031972ecf89dfecd551fd520fafcc))
* **templates:** :sparkles: add suggestion for adding a search subscription to search suggestions auto-complete ([359a0b6](https://github.com/immanent-tech/foragd/commit/359a0b6f7b43c8880b6cc3fe5f67b8b841df36e4))
* **templates:** :sparkles: add umami analytics tracker code in production environments ([6949a15](https://github.com/immanent-tech/foragd/commit/6949a159543723bb2d58dd49848ec1ad668f38a9))
* **templates:** :sparkles: create add group subscription suggestion ([b8829f9](https://github.com/immanent-tech/foragd/commit/b8829f9d3a8e4893142e98b9dd083d0ba37ff93a))
* **templates:** :sparkles: generalise and re-use back to top button ([318faa9](https://github.com/immanent-tech/foragd/commit/318faa9430515cc51d93723dd55e38e90edb6b93))
* **templates:** :sparkles: proxy user avatar in header ([dfe15d4](https://github.com/immanent-tech/foragd/commit/dfe15d411105870197c4bafd1fed08ff96496b15))
* **templates:** :sparkles: share action using share web api with fallback to modal and buttons ([ebdf67b](https://github.com/immanent-tech/foragd/commit/ebdf67be74ae920109857bc7e5f149f70f1afc4f))
* **templates:** :sparkles: show favorites active filter badge as appropriate when listing subscriptions ([fdfbd18](https://github.com/immanent-tech/foragd/commit/fdfbd1884efaf7c43a79a43a5aea19e264e478e1))
* **templates:** :sparkles: show notifications for toggling favorite subscription/article (for user feedback) ([37f5fbe](https://github.com/immanent-tech/foragd/commit/37f5fbec25191e23ff3955f0d977830cb4711172))
* **templates:** :sparkles: use css transitions api for page changes ([c2c652b](https://github.com/immanent-tech/foragd/commit/c2c652b52cc5ee19fb84e60cfeba53a11a8fdc58))
* **templates:** :wrench: set Accept-CH header to get extra client viewport dimension info ([3a5b990](https://github.com/immanent-tech/foragd/commit/3a5b99071a246890fd41948a693b9a9d9529b107))


### Bug Fixes

* :bento: swap dark/light favicons ([2cc2477](https://github.com/immanent-tech/foragd/commit/2cc2477c915f4931a622c8568e7b87051334ced5))
* :bug: avoid version conflicts with rapid index requests to user object ([4bfc1dc](https://github.com/immanent-tech/foragd/commit/4bfc1dcea89def756bc91a9f5127d85caf2573e6))
* :bug: correct search subscription display styling based on dynamic info ([081c8b2](https://github.com/immanent-tech/foragd/commit/081c8b234b556ab43cd4c8ea3cf32b2c32e55277))
* :bug: expose view when editing a search subscription ([7fe6320](https://github.com/immanent-tech/foragd/commit/7fe6320732bb03dfa6db9a51d1ba406ee24919d6))
* :bug: fix account details required ([5df1bb4](https://github.com/immanent-tech/foragd/commit/5df1bb41b0fcd2c1711fa314254917e8ac51a5e8))
* :bug: fix calculating last updated timestamp for subscriptions ([7ce7070](https://github.com/immanent-tech/foragd/commit/7ce7070a986e8d8843574c52f256babc7a44a34f))
* :bug: fix favorite searches ([9159362](https://github.com/immanent-tech/foragd/commit/9159362fc6c347d345e6fff73340e56ee481fc8b))
* :bug: fix marking articles after schema changes ([fd842b4](https://github.com/immanent-tech/foragd/commit/fd842b4b2f8163424ab38e26a48f1fc3465916a2))
* :bug: fix marking group subscription ([e66f9a1](https://github.com/immanent-tech/foragd/commit/e66f9a1d2223660726ff0b84f19f0479f9e46503))
* :bug: fix no search results page ([e245b26](https://github.com/immanent-tech/foragd/commit/e245b26be85b1bcc6df5681ad40effffd55dd530))
* :bug: get all subscription categories for filtering ([88a0212](https://github.com/immanent-tech/foragd/commit/88a02122a9312eede2b1549028896dd8e9564982))
* :bug: handle new user without any favorites clicking around more gracefully ([58e54dc](https://github.com/immanent-tech/foragd/commit/58e54dc9f528a7da1b08ed9dce1bc6e517866514))
* :bug: make sure any subscription filters are populated when editing a search subscription ([f817c3c](https://github.com/immanent-tech/foragd/commit/f817c3c3b7cbb23fdfe4971de53c374bd169c9bf))
* :bug: set proper page titles ([6f58f8d](https://github.com/immanent-tech/foragd/commit/6f58f8defc8e818bf2bb8871b0e89a65985fc755))
* :rewind: switch back to object-fit: cover for responsive media ([56af28f](https://github.com/immanent-tech/foragd/commit/56af28f80116c283df009622eff0a681ff3825f3))
* **auth0:** :bug: better generation of appropriate redirect url after logout ([49e8f90](https://github.com/immanent-tech/foragd/commit/49e8f9023a37d1ccf0f60b065b6e416cd5307bac))
* **auth0:** :bug: separate mgmt and auth api connections ([64acb9e](https://github.com/immanent-tech/foragd/commit/64acb9e995b5f13197a616d7d5e020c05a005a9c))
* **container:** :bug: add run command for scheduler container ([9474840](https://github.com/immanent-tech/foragd/commit/9474840f04dd41c1adf67eef21802b6053d74a4f))
* **elastic:** :bug: ensure subscriptions schema gets migrated when no specific migration is requested ([2ecc92f](https://github.com/immanent-tech/foragd/commit/2ecc92fa90051b47b73fc3cfef59eeb17f01d811))
* **elastic:** :bug: fix filtering on subscription categories ([60fbc6c](https://github.com/immanent-tech/foragd/commit/60fbc6cd5993a4475ed5dc9acb25656e5d5f16c9))
* **elastic:** :bug: fix missing favorite field in user schema ([0442ab2](https://github.com/immanent-tech/foragd/commit/0442ab2c053120adb7305927766a784c689ce16a))
* **elastic:** :bug: handle nil value sort (default to _doc sort) ([933a8e2](https://github.com/immanent-tech/foragd/commit/933a8e2ba2d10332b1843a4776e8d6a97fbd6f22))
* **elastic:** :bug: handle when adding dynamic info involves a subscription that no longer exists ([0e3eab1](https://github.com/immanent-tech/foragd/commit/0e3eab1f1bc7b8cccbf9d2f04217c8e1221beace))
* **handlers:** :bug: don't continue new subscription processing if something failed ([65c012b](https://github.com/immanent-tech/foragd/commit/65c012b9447a74c9942a2e82b7167a35ac15f015))
* **handlers:** :bug: fix errors ([d93a262](https://github.com/immanent-tech/foragd/commit/d93a262f076371136d0f09a0f2c27397bfb99764))
* **handlers:** :bug: fix image proxying not working for some urls ([88b4966](https://github.com/immanent-tech/foragd/commit/88b4966264c790210e1d43382eb6becf686e8188))
* **handlers:** :bug: fix logic for post-login redirection in callback ([3bdd6b7](https://github.com/immanent-tech/foragd/commit/3bdd6b7ae0d7ec6b952cd67a20808fc7c3f5ae97))
* **handlers:** :bug: fix search suggestions ([cf8defd](https://github.com/immanent-tech/foragd/commit/cf8defd1c6fa5b9e08e33565ac691089b14f2ad9))
* **handlers:** :bug: generate correct list of subscriptions for finding article suggestions ([1bc1d3f](https://github.com/immanent-tech/foragd/commit/1bc1d3f402c031b5bc78ddee7f91e39e5ac5e4fe))
* **handlers:** :bug: include any query parameters from original image URL when proxying ([a16334c](https://github.com/immanent-tech/foragd/commit/a16334cdce609807c393feb3d555e04ef1e7c7db))
* **handlers:** :bug: include original query parameters when proxying image ([d881594](https://github.com/immanent-tech/foragd/commit/d8815942da938e47940bf3d09e9c15038315711d))
* **handlers:** :bug: make sure successful results are shown when importing subscriptions ([4f30f25](https://github.com/immanent-tech/foragd/commit/4f30f257902ccf99b99499bc33bca95bea99d99d))
* **handlers:** :bug: saving group subscriptions actually works ([2654a8a](https://github.com/immanent-tech/foragd/commit/2654a8aed398687352d820b5f701232a619f0ddf))
* **handlers:** :bug: when removing a favorite subscription on the favorites page, remove the card ([832350b](https://github.com/immanent-tech/foragd/commit/832350b1cc0c79de11e23b23e5bed1bedf80c0fe))
* **handlers:** :bug: write appropriate status code on image proxy failures ([0109439](https://github.com/immanent-tech/foragd/commit/0109439249b592eac22afbacdb005769441171ce))
* **middlewares:** :bug: don't ratelimit own requests ([e19c256](https://github.com/immanent-tech/foragd/commit/e19c256e21f988e4ec71017560efa1d09a32e741))
* **middlewares:** :bug: handle htmx and non-htmx redirection on user authentication issues ([85201fd](https://github.com/immanent-tech/foragd/commit/85201fda2217bea2ba165a97e384c3d63a2cce26))
* **models:** :bug: always return feed/item source timestamps in UTC ([1cd0b1b](https://github.com/immanent-tech/foragd/commit/1cd0b1ba2ee8c16a8c0d7d06c845318d6314cde9))
* **models:** :bug: compare correct ID when checking if user is already subscribed to a feed ([b703e94](https://github.com/immanent-tech/foragd/commit/b703e9437bba72518031d0f7e3699127636dc862))
* **models:** :bug: correct logic around adding additional subscriptions from group subscriptions ([9f1d7da](https://github.com/immanent-tech/foragd/commit/9f1d7dad9bc12b082d5b24e9a6cd9a7b5c8a876a))
* **models:** :bug: don't sanitise nil filters ([6ca8119](https://github.com/immanent-tech/foragd/commit/6ca8119e86d4b612ebb2217fd1d30cd99243a1fd))
* **models:** :bug: fix generating item ID ([7936bc3](https://github.com/immanent-tech/foragd/commit/7936bc38eebfe966c1c794a2d528a151f39e71ab))
* **models:** :bug: fix getting favorites with a group subscription defined ([49ae7c2](https://github.com/immanent-tech/foragd/commit/49ae7c2e46d2c0a41d79df9912406c9a50ff066a))
* **models:** :bug: fix listing articles ([4dd475e](https://github.com/immanent-tech/foragd/commit/4dd475e7630f78a13cf89d34a65d4b80f19583f5))
* **models:** :bug: fix matching subscriptions for search results ([4d7d9f5](https://github.com/immanent-tech/foragd/commit/4d7d9f5a06b9b596e58be6e5bb9b38dd3afef1ce))
* **models:** :bug: fix new subscription level names ([6aedf8c](https://github.com/immanent-tech/foragd/commit/6aedf8c135e9a5b87da93df57202f59f6260e7ea))
* **models:** :bug: fix pagination of search results ([93357b0](https://github.com/immanent-tech/foragd/commit/93357b099aad620443f9e5df605620efb927bb4f))
* **models:** :bug: fix saving search subscription edits ([1b8cabb](https://github.com/immanent-tech/foragd/commit/1b8cabb17bfda7f2fc401560104de22607481be2))
* **models:** :bug: generate consistent document IDs for items using the source item ID or its URL ([1876921](https://github.com/immanent-tech/foragd/commit/1876921b0ebf23ee8d07b47c883a6e03751fb436))
* **models:** :bug: generate subscription queries appropriately filtering out search/group subscriptions ([c2a9afb](https://github.com/immanent-tech/foragd/commit/c2a9afb712c7d4f3a78730fadfb91504cc9175a4))
* **models:** :bug: handle when feed subscription doesn't have existing article states and article is marked ([76273d5](https://github.com/immanent-tech/foragd/commit/76273d5b3dbeeed1afa6fe249eb5b03870ccf924))
* **models:** :bug: make sure UTC is used for more timestamps ([a2882f5](https://github.com/immanent-tech/foragd/commit/a2882f5a19e4655871f83cdf36ecf1deee5fd41b))
* **models:** :bug: perform post-search filter for subscription favorites ([f29a3f4](https://github.com/immanent-tech/foragd/commit/f29a3f4195cdb3b56453b0797bfaa08a38b62dd5))
* **models:** :bug: remove invalid validation type ([144e9e9](https://github.com/immanent-tech/foragd/commit/144e9e928aa7befe34180e0ccc2ccab87d09b47d))
* **models:** :bug: set last update of feed to latest item timestamp when updating, to handle feeds that lag behind real-time updates ([7158ff3](https://github.com/immanent-tech/foragd/commit/7158ff3f1b4bcf805ca0960dd0baa87abfd7fe40))
* **models:** :bug: subscription fixes ([4907428](https://github.com/immanent-tech/foragd/commit/490742890ed8d63a9eaab746590c9c6804035f16))
* **scheduler:** :bug: ensure check new feeds job and job state documents have unique IDs ([1989a83](https://github.com/immanent-tech/foragd/commit/1989a833f05259fa552eb241f1c3061fb22cb753))
* **scheduler:** :bug: fix check for new feeds state ([0c152d1](https://github.com/immanent-tech/foragd/commit/0c152d1a1a0c13fc85956eb1019a144beced13c8))
* **scheduler:** :bug: fix logic for checking for new feeds and scheduling update jobs ([bb5e289](https://github.com/immanent-tech/foragd/commit/bb5e289103889298b72ea3c65c7a81770fecdfc2))
* **scheduler:** :bug: use correct index aliases for read/write requests ([6588830](https://github.com/immanent-tech/foragd/commit/65888308e3d790d38be3f7b84955b7e5929c6299))
* **templates:** :bug: allowing showing search results when user hits enter after entering text in the global search bar ([2c3f726](https://github.com/immanent-tech/foragd/commit/2c3f7264896de607f9d9f4d44cc7d27c4f1a16af))
* **templates:** :bug: clip article summary content on home page to avoid overflowing content ([9b48ddd](https://github.com/immanent-tech/foragd/commit/9b48ddd066c3f75c942e3361347889a2fc202937))
* **templates:** :bug: close search filters dialog when search is submitted ([0165477](https://github.com/immanent-tech/foragd/commit/016547722f71a887537130c394e40c958775cfe3))
* **templates:** :bug: correct actions display depending on subscription type on list page ([7489d0b](https://github.com/immanent-tech/foragd/commit/7489d0b968c47232499ec6537160a0644e8e5804))
* **templates:** :bug: create prose-summary utility class to control display of card summaries ([d391284](https://github.com/immanent-tech/foragd/commit/d39128464475061bd06a9ac67fdf78cd8a40e2be))
* **templates:** :bug: display group subscription name on article list when viewing articles from a group subscription ([aa93c81](https://github.com/immanent-tech/foragd/commit/aa93c81a91cc060afae30c4dfc4a8ee6ef0315cb))
* **templates:** :bug: don't show category filter button on desktop if no categories to filter on ([3613221](https://github.com/immanent-tech/foragd/commit/361322119bb071bd2c136fe60338fbd40fe66709))
* **templates:** :bug: don't try to use a subscription as the page title if articles slice is empty ([b4f0f17](https://github.com/immanent-tech/foragd/commit/b4f0f178f7025227db1b767744389a6cc725faff))
* **templates:** :bug: ensure search filters dialog never grows bigger than 90% of screen height ([96fb42d](https://github.com/immanent-tech/foragd/commit/96fb42de290252f170393ba603b49ec6b53c83ec))
* **templates:** :bug: fix action links in search suggestions ([693f4d0](https://github.com/immanent-tech/foragd/commit/693f4d01cd0b1580ad85bdbeb5901317058fedbd))
* **templates:** :bug: fix adding a search subscription ([ea750cb](https://github.com/immanent-tech/foragd/commit/ea750cbed6e8a71deeca44ed495d6c39a0ef074b))
* **templates:** :bug: fix adding a search subscription from search suggestions ([d94c957](https://github.com/immanent-tech/foragd/commit/d94c957a2e669af2b6d69409a042e7e6758edf79))
* **templates:** :bug: fix alignment of back to top button on tablet screens ([e44e38b](https://github.com/immanent-tech/foragd/commit/e44e38b07b14beb0355ce5a3d1a7890f2862f7cf))
* **templates:** :bug: fix another mark articles action ([0a6667c](https://github.com/immanent-tech/foragd/commit/0a6667ca506fa72331a65324173eaf7d539de8d3))
* **templates:** :bug: fix edit subscription after subscription refactoring ([2e414af](https://github.com/immanent-tech/foragd/commit/2e414af27403944dafdd2b111e5f1c16d38febef))
* **templates:** :bug: fix marking a subscription on favorites page ([8776396](https://github.com/immanent-tech/foragd/commit/87763960db472318c2060afd62fbf30c356c50ed))
* **templates:** :bug: fix sorting not working after splitting list objects handler ([ce1f7ae](https://github.com/immanent-tech/foragd/commit/ce1f7aee48bdead4be937cef8fb9d907c0d07472))
* **templates:** :bug: fix toggling mark when viewing article ([372115f](https://github.com/immanent-tech/foragd/commit/372115f6c6f0e4a450344865a747307a14489a02))
* **templates:** :bug: include mark as form value when marking all subscriptions ([56e4252](https://github.com/immanent-tech/foragd/commit/56e4252c56313c1605a2a56876a4fd80f8b49972))
* **templates:** :bug: more selective hx-include on edit subscription form ([4e90585](https://github.com/immanent-tech/foragd/commit/4e90585e5818883bedd9070505cfff3dbab7c01c))
* **templates:** :bug: on back button action, restore the page scroll state appropriately ([43c7a94](https://github.com/immanent-tech/foragd/commit/43c7a94c1273fd235dc531b975c00050a44c04fa))
* **templates:** :bug: pass subscription ID for subscription actions ([26f511f](https://github.com/immanent-tech/foragd/commit/26f511fdbb684098bc98da875c4254e74a4737c4))
* **templates:** :bug: pass subscription title as appropriate ([e6d2af6](https://github.com/immanent-tech/foragd/commit/e6d2af63217bcdbd65f788f7c54fdc9b3b0c9f5c))
* **templates:** :bug: re-add modal for sharing an article when share api is not available ([a33a877](https://github.com/immanent-tech/foragd/commit/a33a8772663789e08c17cd65356c576bd349f134))
* **templates:** :bug: remove extra character ([27f2885](https://github.com/immanent-tech/foragd/commit/27f2885165523a8fe9c99830ab280c6f9aa76503))
* **templates:** :bug: search fixes ([855f5a0](https://github.com/immanent-tech/foragd/commit/855f5a0483ae275ca6330fabfaa4e0122e81671d))
* **templates:** :bug: show appropriate actions on articles page based on display ([42c6647](https://github.com/immanent-tech/foragd/commit/42c6647fe6edf682ff75041ccea187b2dd5f491c))
* **templates:** :bug: show list controls even when no objects are shown ([a5bdaf6](https://github.com/immanent-tech/foragd/commit/a5bdaf653f551c40e7b6b78b548529fb189e7b03))
* **templates:** :bug: show top of window when navigating between subscriptions/articles ([cfb1d4d](https://github.com/immanent-tech/foragd/commit/cfb1d4dfc8a172bfe07b3df48c2344bcdf4a5c6f))
* **templates:** :bug: use forms for all list control actions ([8238ffe](https://github.com/immanent-tech/foragd/commit/8238ffe5e1614e0371a03f3e9405d6745b6fd055))
* **templates:** :bug: when transitioning to a new page, show the top of the page ([d32b8f3](https://github.com/immanent-tech/foragd/commit/d32b8f36ca1d88c24562146cf2a787d9da1b7df7))
* **templates:** :bug: when viewing, clicking article title should go to external site ([1353c31](https://github.com/immanent-tech/foragd/commit/1353c31409ecb7d4c40cc8446efc6fe9cb3786ae))
* **templates:** :fire: remove debugging code ([e469467](https://github.com/immanent-tech/foragd/commit/e4694679bf0e99236bc90de4b6240d95a9053d7d))
* **templates:** :lipstick: consistent image sizing on article cards ([078679e](https://github.com/immanent-tech/foragd/commit/078679e08b29501889e604b214253ac419ff8a5f))
* **templates:** :lipstick: fix display of article and updated date in various article layouts ([cfaff59](https://github.com/immanent-tech/foragd/commit/cfaff592db70fbc763d1dd5f35de72b29bd5b456))
* **templates:** :lipstick: fix effects on screenshot on landing ([dc3b030](https://github.com/immanent-tech/foragd/commit/dc3b03079499a5f763b4f20d653f658673abefeb))
* **templates:** :lipstick: fix update notification buttons being unreadable ([3cc9449](https://github.com/immanent-tech/foragd/commit/3cc94490c24c38e5f3b38b83a88148dac527c458))
* **templates:** :lipstick: fix width of report page issue form ([3c96a66](https://github.com/immanent-tech/foragd/commit/3c96a664fc65a833ca680fccbae58610ed414ee8))
* **templates:** :lipstick: show top of article when clicking its link from an article card ([20e99c2](https://github.com/immanent-tech/foragd/commit/20e99c2b08df2a3a1e25f65ffcc8a9bb1c5a808f))
* **templates:** :lipstick: truncate long subscription titles ([d51daf2](https://github.com/immanent-tech/foragd/commit/d51daf2085755bc72210d1930eeba883b0327ede))
* **templates:** :speech_balloon: fix spelling and grammar on landing page ([64a047a](https://github.com/immanent-tech/foragd/commit/64a047ac58684e132956ee8bbb693def1258729a))


### Performance Improvements

* :zap: better image proxy caching ([d2a532f](https://github.com/immanent-tech/foragd/commit/d2a532f7297b6c0f37979ca2ddb60f0a5fd19ac2))
* **auth0:** :zap: defer connecting auth0 authentication service until needed ([e1189f0](https://github.com/immanent-tech/foragd/commit/e1189f0199d9862a59d0501985e981caa1702eab))
* **elastic:** :zap: alwasys refresh when a bulk operation involves doc updates ([c3d558c](https://github.com/immanent-tech/foragd/commit/c3d558c4f12712ada9b484e6aa98c2b72c64ebe7))
* **elastic:** :zap: simplify elastic client set up ([91c1093](https://github.com/immanent-tech/foragd/commit/91c1093ecfeaed79d6d4d9eff8b62ae46df117c6))
* **github:** :zap: defer connecting to github until needed, then cache the connection ([02b19e1](https://github.com/immanent-tech/foragd/commit/02b19e1975002ca4c679c1a16c8a6e7086e5ebd3))
* **handlers:** :zap: set Cache-Control response header on list/home pages ([7b1d06d](https://github.com/immanent-tech/foragd/commit/7b1d06da0609662fb100650c799bf5704e3de820))
* **models:** :zap: boost documents closer to current time when searching ([6242aa3](https://github.com/immanent-tech/foragd/commit/6242aa372193d714ed17ef4c5879b2881455785d))
* **models:** :zap: default search within period set to last week ([d34f35c](https://github.com/immanent-tech/foragd/commit/d34f35c5560b0b45274066cf37e638fb9ea374d8))
* **models:** :zap: optimise number of results returned for search/suggestions for better layout and performance ([412b42c](https://github.com/immanent-tech/foragd/commit/412b42c338f340cd72d6f11d886a89c7e5467b01))
* **server:** :zap: compress svg ([6263eba](https://github.com/immanent-tech/foragd/commit/6263eba16492c8ed76c614993ac8b21ce7a52763))
* **templates:** :zap: always proxy subscription avatar images ([71ecc37](https://github.com/immanent-tech/foragd/commit/71ecc37cdfa37e3d027e87537d75561c3b22d9ac))
* **templates:** :zap: preload images for pagination requests ([7e37f4e](https://github.com/immanent-tech/foragd/commit/7e37f4e24d92c42260fe3a47eb5c4fef0861e987))
* **templates:** :zap: remove unused event tracking on grid container ([7c3efb9](https://github.com/immanent-tech/foragd/commit/7c3efb99ee5cd733bbab349cb9d170a56fb49862))


### Miscellaneous Chores

* release 0.0.1 ([9829303](https://github.com/immanent-tech/foragd/commit/98293034ec8746022ac9e4a8f9f0fd39edc91e94))
