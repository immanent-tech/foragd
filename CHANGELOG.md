# Changelog

## [0.158.0](https://github.com/immanent-tech/foragd/compare/v0.157.0...v0.158.0) (2026-06-10)


### Features

* ✨ add form to request deletion of account and personal data ([b48c322](https://github.com/immanent-tech/foragd/commit/b48c322c907260d34712072f9298f01fccd484d2))
* **pwa:** ✨ additional pwa/app feature support ([f81f507](https://github.com/immanent-tech/foragd/commit/f81f507595d41f6f86ba0112efc06f8ab2cff7d0))
* **templates/articles:** 🎨 improved article card styling ([3a4b1ad](https://github.com/immanent-tech/foragd/commit/3a4b1adc6b629cf937315bdf76936f70745e7200))
* **templates/home:** 🎨 improved home page layout ([8f1dca6](https://github.com/immanent-tech/foragd/commit/8f1dca6f6d7e4ddbfe39709b2110beae55f0072b))


### Bug Fixes

* **android:** 🔧 fix twa-manifest format ([07142b2](https://github.com/immanent-tech/foragd/commit/07142b2eaab032cecd80ef1b19b31a80aef28811))
* **gcp:** 🐛 fix init of error client ([67c1024](https://github.com/immanent-tech/foragd/commit/67c10241acc79d4ec84e758eaac137a58c52cdfb))
* **paddle:** 🐛 fix handling of new subscription ([e166625](https://github.com/immanent-tech/foragd/commit/e1666257e328f1a021a81c54e33bef61726ab094))
* **server:** ♻️ enable HTTP/1.1 ([80c281b](https://github.com/immanent-tech/foragd/commit/80c281b3570d412f247f319b74a6c6aa345437c6))
* **server:** ♻️ update and removed deprecated h2c server configuration ([8850094](https://github.com/immanent-tech/foragd/commit/8850094717d88a383611b9e5311865c7ee47b187))
* **server:** 🔧 fix forget-me handling ([d5c6580](https://github.com/immanent-tech/foragd/commit/d5c658005c86731c27cde4f107b845f627d9e9e7))
* **server:** 🔧 fix forget-me handling ([eab6a13](https://github.com/immanent-tech/foragd/commit/eab6a137de672c9317a3084daa082a95adb2eac7))
* **service/feeds:** 🐛 fix defer of response body close ([cb31870](https://github.com/immanent-tech/foragd/commit/cb318702dca4fef83dd97d5548e3b56f923217c6))

## [0.157.0](https://github.com/immanent-tech/foragd/compare/v0.156.0...v0.157.0) (2026-06-09)


### Features

* ✨ add initial (disabled) android billing integration ([cae307d](https://github.com/immanent-tech/foragd/commit/cae307d63650de27bc868717db8ae6cef0704c38))

## [0.156.0](https://github.com/immanent-tech/foragd/compare/v0.155.2...v0.156.0) (2026-06-08)


### Features

* **templates/header:** ✨ add changelog link and icon styling ([1f66fc7](https://github.com/immanent-tech/foragd/commit/1f66fc7414a45fb17831d186936409979a2334e6))
* **templates/home:** ✨ improve new user home page ([fda8623](https://github.com/immanent-tech/foragd/commit/fda8623a954a57c35c492a90e68d73c9d2f2608f))
* **templates/subscriptions:** ✨ improve add subscription page ([e49742b](https://github.com/immanent-tech/foragd/commit/e49742be07a5f93a3879d660774cf4851dd69cc2))

## [0.155.2](https://github.com/immanent-tech/foragd/compare/v0.155.1...v0.155.2) (2026-06-08)


### Bug Fixes

* **handlers/user:** 🔧 fix display of subscriptions settings page when user has no subscriptions ([25ac774](https://github.com/immanent-tech/foragd/commit/25ac774929ad7c17285d39949aa68b96c0fd738b))
* **server/otel:** 🔧 don't add authorization header for non-production deployments ([3e91f76](https://github.com/immanent-tech/foragd/commit/3e91f76d95872bdff64887cd4c9e7fb18766de78))
* **server:** 🔧 set otel authorization headers ([17f9f73](https://github.com/immanent-tech/foragd/commit/17f9f738bc140eac8109c5799d088330c0ad71ca))

## [0.155.1](https://github.com/immanent-tech/foragd/compare/v0.155.0...v0.155.1) (2026-06-05)


### Bug Fixes

* **config:** 🐛 make sure version is parsed from environment ([576bb05](https://github.com/immanent-tech/foragd/commit/576bb058ad2fa7f1ec47cc001d58384df39d2708))

## [0.155.0](https://github.com/immanent-tech/foragd/compare/v0.154.0...v0.155.0) (2026-06-05)


### Features

* **pkg/formats/html:** ✨ add method to strip html back to plain text ([9450389](https://github.com/immanent-tech/foragd/commit/9450389df8b947e25f40ac655826511e2072ba41))
* **templates/features:** ♻️ improved features page ([c77e6d2](https://github.com/immanent-tech/foragd/commit/c77e6d2e0fc4ba4584923308dd9bed1fb69bfde8))
* **templates:** ✨ add ItemList JSON-LD schema ([0e5a179](https://github.com/immanent-tech/foragd/commit/0e5a179baed58cc2f389402802c4dbe5d9a1ba07))
* **templates:** 🎨 more vibrant notifications ([61530a5](https://github.com/immanent-tech/foragd/commit/61530a52d9686a713ec490ada7e1e88b836ea67d))


### Bug Fixes

* **service/users:** 🔧 don't update for limit checks if not needed ([c32b920](https://github.com/immanent-tech/foragd/commit/c32b9207f8a59c779eb7f3f4bc8c2779a5ef7f3d))

## [0.154.0](https://github.com/immanent-tech/foragd/compare/v0.153.0...v0.154.0) (2026-06-04)


### Features

* ✨ easy way to add google news rss subscriptions ([5caa6c5](https://github.com/immanent-tech/foragd/commit/5caa6c580ec25a8a5767b1376a8157b1424c0c8f))
* **middlewares:** ✨ add middleware to detect client access (pwa, twa or web) ([55386a8](https://github.com/immanent-tech/foragd/commit/55386a8b0075755f225d8996bb767189df548b27))

## [0.153.0](https://github.com/immanent-tech/foragd/compare/v0.152.1...v0.153.0) (2026-06-03)


### Features

* ✨ add a user-facing changelog ([26e7495](https://github.com/immanent-tech/foragd/commit/26e74950245f5b64d3efd4bf65abfeb94d00222b))
* **blog:** ✨ new post and some clean ups ([30347c0](https://github.com/immanent-tech/foragd/commit/30347c08fabe7c88fbef5feb09ecb8d0211abd79))

## [0.152.1](https://github.com/immanent-tech/foragd/compare/v0.152.0...v0.152.1) (2026-06-03)


### Bug Fixes

* **models:** 🔧 fix email newsletter limit! ([a43e088](https://github.com/immanent-tech/foragd/commit/a43e088b8ce8697a2d6f012673e9a95606bfa769))

## [0.152.0](https://github.com/immanent-tech/foragd/compare/v0.151.1...v0.152.0) (2026-06-03)


### Features

* ✨ dynamically generate feedsets ([679c6ea](https://github.com/immanent-tech/foragd/commit/679c6ea1a480d778d694760ba5f0bc8cb3971f01))
* **email:** ♻️ add source-based utm tags to emails ([7767aab](https://github.com/immanent-tech/foragd/commit/7767aabe30ad5ef48fbe1ba5f185b5d4f8afcc60))
* **templates:** 🎨 expose more theme suggestions ([4be0fee](https://github.com/immanent-tech/foragd/commit/4be0fee99a26d4e4176004a1599bf77051aed9be))


### Bug Fixes

* **service:** 🔧 add missing error check ([daffc46](https://github.com/immanent-tech/foragd/commit/daffc46f7b171411cf0836fdd75a3bdf3596dc5c))
* **subscriptions:** ♻️ improved opml export logic ([a7be295](https://github.com/immanent-tech/foragd/commit/a7be2957335f545a4939ee53d123e47c4d9708fd))

## [0.151.1](https://github.com/immanent-tech/foragd/compare/v0.151.0...v0.151.1) (2026-06-02)


### Bug Fixes

* **templates/search:** 🔧 fix focusing global search ([0bf8e85](https://github.com/immanent-tech/foragd/commit/0bf8e85035d5bd2da5d86d533b24e7887ae3a255))

## [0.151.0](https://github.com/immanent-tech/foragd/compare/v0.150.1...v0.151.0) (2026-06-02)


### Features

* ✨ better account limits handling ([6493b35](https://github.com/immanent-tech/foragd/commit/6493b357ab309fe2045d2fd0c7d66984b7d40a82))


### Bug Fixes

* 🔧 better handling of updated items ([e0ff2ee](https://github.com/immanent-tech/foragd/commit/e0ff2ee9e90698126c867c5524f77104396ca934))
* **models:** 🔧 fix subscription limit ([66e9607](https://github.com/immanent-tech/foragd/commit/66e9607af85d049ce0c5c0417bb6d15a13df4ab1))

## [0.150.1](https://github.com/immanent-tech/foragd/compare/v0.150.0...v0.150.1) (2026-05-29)


### Performance Improvements

* **server:** ⚡ add instrumentation to user methods ([653cc1e](https://github.com/immanent-tech/foragd/commit/653cc1e622d6a17ebb9a3fd25f892fbc39fff3ff))

## [0.150.0](https://github.com/immanent-tech/foragd/compare/v0.149.0...v0.150.0) (2026-05-29)


### Features

* **templates:** ✨ support specifying template fragments for generic partial render ([1065e8f](https://github.com/immanent-tech/foragd/commit/1065e8f7c53714a4576504821b2bafb47dfaceca))
* **user/settings:** ✨ provide a settings tab for bulk management of subscriptions ([3bb6722](https://github.com/immanent-tech/foragd/commit/3bb6722c504b2290e5a2bec2912e50852f76eeb4))


### Performance Improvements

* **server:** ⚡ add some instrumentation to measure some important code paths ([fa2fab8](https://github.com/immanent-tech/foragd/commit/fa2fab8bf004b9a45c337aaf8b28cd4606aab07f))

## [0.149.0](https://github.com/immanent-tech/foragd/compare/v0.148.1...v0.149.0) (2026-05-27)


### Features

* **server:** ♻️ rework OTEL instrumentation ([019db47](https://github.com/immanent-tech/foragd/commit/019db47aed7c3b7b1265c2bca3d3026e8051bbbe))

## [0.148.1](https://github.com/immanent-tech/foragd/compare/v0.148.0...v0.148.1) (2026-05-27)


### Bug Fixes

* 🔧 fix logo link ([335d4b2](https://github.com/immanent-tech/foragd/commit/335d4b2fb3fe44fcd4d7fcdfce3bdcdc737888f7))
* **service:** 🔧 feed fetching improvements ([ddfece9](https://github.com/immanent-tech/foragd/commit/ddfece91671a6bdd27972494da5fa1be3726d8c1))

## [0.148.0](https://github.com/immanent-tech/foragd/compare/v0.147.1...v0.148.0) (2026-05-27)


### Features

* **models:** ✨ add quirks tracking for feeds ([0f26c1b](https://github.com/immanent-tech/foragd/commit/0f26c1b6dd5f5868152d8fb939d5c76a02957f6f))
* **models:** ✨ update GetContent method for articles to handle more article types ([bb751a4](https://github.com/immanent-tech/foragd/commit/bb751a41a3bd918953c15eae0e6d18a532dd5e42))
* **templates:** 🎨 adjust cards layout ([f0ee879](https://github.com/immanent-tech/foragd/commit/f0ee87932bdf75981f8f6c6404bef630257fceb7))
* **zyte:** ✨ perform own article extraction ([9221ddd](https://github.com/immanent-tech/foragd/commit/9221ddd411cf4befff87c47b7ca9d311b179d58d))


### Bug Fixes

* **service:** 🐛 fix error logic in FetchFeed ([36febd0](https://github.com/immanent-tech/foragd/commit/36febd049cd4286e63251e51d6465a23a4e4da24))
* **service:** 🔧 make sure url for extracted image points to an actual image file ([f36beb5](https://github.com/immanent-tech/foragd/commit/f36beb55c87b4c0dcedd73ecaf818f27fb4648f4))

## [0.147.1](https://github.com/immanent-tech/foragd/compare/v0.147.0...v0.147.1) (2026-05-26)


### Bug Fixes

* 🔧 fix request logic ([8150b87](https://github.com/immanent-tech/foragd/commit/8150b87fd69c2dfe7b5253f30fe04d1aff01bfc5))
* **service:** 🔧 ensure feed URL is passed through extra parser ([d8d6247](https://github.com/immanent-tech/foragd/commit/d8d62475d0e5d58d5979d5600e3ca6a513459afb))


### Performance Improvements

* **service:** 🔧 optimise when extra info is fetched for feeds/items ([ca02865](https://github.com/immanent-tech/foragd/commit/ca028655795d8c0621bd6e3a667c332058687316))

## [0.147.0](https://github.com/immanent-tech/foragd/compare/v0.146.0...v0.147.0) (2026-05-26)


### Features

* ✨ feed fetching tweaks ([ae28ff2](https://github.com/immanent-tech/foragd/commit/ae28ff21b311d70d625d62e416d35eae073dc633))
* ✨ use zyte to fetch remote article content ([7439d55](https://github.com/immanent-tech/foragd/commit/7439d55b10f50994855f743397cbdffc60d8e405))
* **service:** ✨ use Zyte as a proxy for downloading feed data where necessary ([9d4382d](https://github.com/immanent-tech/foragd/commit/9d4382dcffb1589c03af420de252fd5be5069e21))
* **zyte:** ✨ add zyte API provider package ([649debe](https://github.com/immanent-tech/foragd/commit/649debeddd8bcbb5f9dee11a350c066ee13665ae))


### Bug Fixes

* **templates/landing:** 🔧 use relative path to images in blog posts on landing page ([92e1f9d](https://github.com/immanent-tech/foragd/commit/92e1f9d113e2f96ff6db29a0260db8bc4d7970c2))

## [0.146.0](https://github.com/immanent-tech/foragd/compare/v0.145.0...v0.146.0) (2026-05-22)


### Features

* **templates/features:** 🎨 improved tab transitions ([8b5eb6d](https://github.com/immanent-tech/foragd/commit/8b5eb6daf639e5f75fe7a8636ddc5e11885fdb72))
* **templates/subscriptions:** 🎨 better fade out when marking subscription/article cards as read ([da3c51a](https://github.com/immanent-tech/foragd/commit/da3c51a6d9e5c8f87f3f81bd171b93cdedbc3be2))


### Bug Fixes

* **assets:** 🔧 fix color contrast on catppuccin themes ([59a8865](https://github.com/immanent-tech/foragd/commit/59a88659beaae5cedc794a24880f7bca200e13e5))

## [0.145.0](https://github.com/immanent-tech/foragd/compare/v0.144.1...v0.145.0) (2026-05-22)


### Features

* **email:** ✨ send an email close to trial expiry as a reminder ([75e0c19](https://github.com/immanent-tech/foragd/commit/75e0c199649f5912d8c3d36878c2f76d2e1822aa))


### Bug Fixes

* **favorites,subscriptions:** 🔧 fix crash when viewing favorites with a group subscription ([f0fe0dc](https://github.com/immanent-tech/foragd/commit/f0fe0dc99bf3d5f3c863f7091453993381f16647))

## [0.144.1](https://github.com/immanent-tech/foragd/compare/v0.144.0...v0.144.1) (2026-05-22)


### Bug Fixes

* **policies:** 🔧 update table of contents ([8b725bc](https://github.com/immanent-tech/foragd/commit/8b725bc183a14da7777801e1430dbd2227e1ccb2))
* **templates/checkout:** 🔧 check both trial and active subscription status ([73984f2](https://github.com/immanent-tech/foragd/commit/73984f26834b062b075b6b2f4f57cd5a79611ea1))

## [0.144.0](https://github.com/immanent-tech/foragd/compare/v0.143.1...v0.144.0) (2026-05-22)


### Features

* **favorites:** ✨ display latest articles on subscription cards on favorites page ([fed62a8](https://github.com/immanent-tech/foragd/commit/fed62a86f0ca3fe2fe42ca6613b04b3f82e15083))


### Bug Fixes

* **config:** 🔧 mispelt variable ([08dd86b](https://github.com/immanent-tech/foragd/commit/08dd86bc6875ed85dde03ce7c37f78de8795a901))

## [0.143.1](https://github.com/immanent-tech/foragd/compare/v0.143.0...v0.143.1) (2026-05-21)


### Bug Fixes

* 🔧 remove mentions of beta (now that app is out of beta) ([c2738f2](https://github.com/immanent-tech/foragd/commit/c2738f2343615d8180b661987bfec97897cd586f))
* **email:** 🎨 fix spacing in 3-to-1 grid component ([a02a085](https://github.com/immanent-tech/foragd/commit/a02a08547be7380a2e65b2f30cf8fe0542898436))
* **templates:** 🎨 adjust top margin on internal error pages ([4cf1324](https://github.com/immanent-tech/foragd/commit/4cf1324c584cb1c8d1a42bc9ceccdddb3014117c))

## [0.143.0](https://github.com/immanent-tech/foragd/compare/v0.142.5...v0.143.0) (2026-05-21)


### Features

* ✨ paddle integration ([8825166](https://github.com/immanent-tech/foragd/commit/882516620116c3cf09eb6d3fdc2f404aff52c80a))

## [0.142.5](https://github.com/immanent-tech/foragd/compare/v0.142.4...v0.142.5) (2026-05-20)


### Bug Fixes

* **models:** 🐛 add new no dynamic mapping component template ([caf69ee](https://github.com/immanent-tech/foragd/commit/caf69ee88fb29c3058a21b0440f2b75539dcbcef))
* **scheduler:** 🐛 fix error message arguments ([4731c81](https://github.com/immanent-tech/foragd/commit/4731c8173370d1f078a465739b5775f2ed7ca68d))
* **service:** 🔧 guard against empty slices when calculating dynamic info for subscription types ([c57f75d](https://github.com/immanent-tech/foragd/commit/c57f75d6cd2dc6b9216c6acf313975788feebefe))


### Performance Improvements

* **scheduler:** ⚡ tweak pagination for retrieving all jobs from scheduler queue ([e0ab314](https://github.com/immanent-tech/foragd/commit/e0ab31427191bf0c61b95fad5f745ddad3f15826))

## [0.142.4](https://github.com/immanent-tech/foragd/compare/v0.142.3...v0.142.4) (2026-05-19)


### Bug Fixes

* **elastic:** 🔧 support more values of dynamic mapping index template setting ([5ee16c0](https://github.com/immanent-tech/foragd/commit/5ee16c09d91ed0483f99bccccaac37b347ef6b61))
* **scheduler:** 🔧 remove debugging code ([99e51fb](https://github.com/immanent-tech/foragd/commit/99e51fb94974fd9e4116356bf135a89e06478241))
* **templates:** 🔧 allow category filters to include subscription filters ([846845f](https://github.com/immanent-tech/foragd/commit/846845f695c944749c860f113dfe0635e0d70b35))
* **templates:** 🔧 make sure notifications toast is above other content ([beba2bc](https://github.com/immanent-tech/foragd/commit/beba2bcfdd51392b144c3b25f8a7b5ef0a242a97))

## [0.142.3](https://github.com/immanent-tech/foragd/compare/v0.142.2...v0.142.3) (2026-05-19)


### Bug Fixes

* **scheduler:** ♻️ vastly improved scheduler logic ([b245aa2](https://github.com/immanent-tech/foragd/commit/b245aa22a068df3fd848950625319e6a46852f10))

## [0.142.2](https://github.com/immanent-tech/foragd/compare/v0.142.1...v0.142.2) (2026-05-17)


### Bug Fixes

* 🔧 clean up code related to fetching feed data ([a35f705](https://github.com/immanent-tech/foragd/commit/a35f7054899850450611e08093e90ef8fdff2a6a))
* **articles:** 🔧 fix youtube video display ([3829975](https://github.com/immanent-tech/foragd/commit/3829975dbf93067000ae0d65cd99f259c935adbc))
* **subscriptions:** 🔧 fix pagination triggers going nuts on firefox due to intersection observer API quirks ([c50afbc](https://github.com/immanent-tech/foragd/commit/c50afbc54d3ad1f5130e5e0457a3a13b2f6d7f8c))

## [0.142.1](https://github.com/immanent-tech/foragd/compare/v0.142.0...v0.142.1) (2026-05-15)


### Bug Fixes

* **subscriptions:** 🔧 fix cache issues with subscriptions ([9c22341](https://github.com/immanent-tech/foragd/commit/9c223415faa34506459d88b6e4ebcf607d66ed25))
* **subscriptions:** 🔧 get subscriptions by id or get all subscriptions in some places ([adf3a00](https://github.com/immanent-tech/foragd/commit/adf3a00ea674a0fd282bfd31434738290b8feff9))

## [0.142.0](https://github.com/immanent-tech/foragd/compare/v0.141.0...v0.142.0) (2026-05-15)


### Features

* **handlers:** ✨ add referer to internal error ([5fbe593](https://github.com/immanent-tech/foragd/commit/5fbe5930e1685a161e26657e35e6c5576ef69137))


### Bug Fixes

* **styles:** 🔧 don't set any themes other than greenhouse as default for either light or dark preferences ([ad8d3de](https://github.com/immanent-tech/foragd/commit/ad8d3de29cdc589524d192d0547c3d464403e3f8))


### Performance Improvements

* **home:** ⚡ greatly increase home page generation ([4f4f593](https://github.com/immanent-tech/foragd/commit/4f4f593264789e7106b8d059119880df811c57ab))

## [0.141.0](https://github.com/immanent-tech/foragd/compare/v0.140.0...v0.141.0) (2026-05-15)


### Features

* **templates:** ♻️ combine list views categories and view menus into single filters menu on desktop ([b67d655](https://github.com/immanent-tech/foragd/commit/b67d655204ea4e09fe2d31a2b8f2674852ac0209))


### Bug Fixes

* **email:** 🎨 lots of fixes to email templates ([ce7118b](https://github.com/immanent-tech/foragd/commit/ce7118b3bcf19e16aa85d8559c495b2f278fa2d9))
* **email:** 🔧 fix some element layouts in email templates ([187107a](https://github.com/immanent-tech/foragd/commit/187107a38724ec107b109c5747c195051c86f26a))


### Performance Improvements

* **handlers:** 🔧 add additional logging for performance metrics ([90092f1](https://github.com/immanent-tech/foragd/commit/90092f1d47ae5d993439bbd2fa0bbf13bd2f3fd9))

## [0.140.0](https://github.com/immanent-tech/foragd/compare/v0.139.0...v0.140.0) (2026-05-15)


### Features

* ✨ add some more logo/icon variants ([fc8a72b](https://github.com/immanent-tech/foragd/commit/fc8a72b05fe1817626fedafb0c8d0c7f54c865df))
* **email:** 🎨 update email templates for new theme/style ([2856bab](https://github.com/immanent-tech/foragd/commit/2856bab8b8adc447a906c25a4ee116e980dea4fc))


### Bug Fixes

* **email:** 🎨 make email theme consistent with default web theme ([e7635e0](https://github.com/immanent-tech/foragd/commit/e7635e07fa419be28f9e54f54b2d740afefe660a))

## [0.139.0](https://github.com/immanent-tech/foragd/compare/v0.138.0...v0.139.0) (2026-05-14)


### Features

* ✨ new logo and icon ([1dc3aff](https://github.com/immanent-tech/foragd/commit/1dc3aff8e08ac1207f34778e96aa9e833f4bf503))


### Bug Fixes

* **articles:** 🎨 add a small amount of padding to article content on small screens ([8bd60e3](https://github.com/immanent-tech/foragd/commit/8bd60e33e86366927615a176de9ceb74c86fd992))
* **articles:** 🎨 fix subscription name/last updated wrapping on article view ([84326bf](https://github.com/immanent-tech/foragd/commit/84326bf25f5a268fcd58245369f6bc34afc53e73))
* **feeds:** 🔧 explicitly sort when retrieving latest items ([164f475](https://github.com/immanent-tech/foragd/commit/164f475457aee1414bb99b34b3d0736a74fca7f7))

## [0.138.0](https://github.com/immanent-tech/foragd/compare/v0.137.0...v0.138.0) (2026-05-14)


### Features

* **models:** :sparkles: add additional details for youtube videos ([0e0896c](https://github.com/immanent-tech/foragd/commit/0e0896cc2dfc4330ad0c903c8f02beebc10ea075))
* **templates:** :lipstick: show an animated loading indicator on toast action ([3fd8687](https://github.com/immanent-tech/foragd/commit/3fd86877a79904b71cf5878b294ac733c876ab92))


### Bug Fixes

* **assets:** :bug: fix 1024px logo to ensure it has padding for any masks that might be applied (i.e., in pwa/twa) ([c691476](https://github.com/immanent-tech/foragd/commit/c691476dde643eb572f59f7847e88b7534cb7eba))
* **cli:** fix url used for fetching items for existing feed ([d233a7b](https://github.com/immanent-tech/foragd/commit/d233a7b9056aa22ad285904c0d1c0f95e3662aa6))
* **feeds:** fix index for feed updates 🫨 ([d15d211](https://github.com/immanent-tech/foragd/commit/d15d211c88100784c2f107709ed54d09b4424ef3))
* **posts:** :bug: fix path in rss feed to posts ([c26aabb](https://github.com/immanent-tech/foragd/commit/c26aabbd0c4f9864733cefaf150020e61b262f76))
* **styles:** :bug: indicate greenhouse theme as default ([4acd411](https://github.com/immanent-tech/foragd/commit/4acd411dd1bdbe15e27697e3e021218e8aa96484))


### Performance Improvements

* **templates:** :zap: move cloudflare turnstile script to only activate on pages where it is used ([0099841](https://github.com/immanent-tech/foragd/commit/00998418ea74dde325d6c4f639aa77fbde9d7a31))

## [0.137.0](https://github.com/immanent-tech/foragd/compare/v0.136.1...v0.137.0) (2026-05-13)


### Features

* **templates:** :lipstick: better menu styling for list controls ([2ee5209](https://github.com/immanent-tech/foragd/commit/2ee5209c3a1e70ea6d23cbb379d2c1d10f4cda9a))


### Performance Improvements

* **service:** :zap: cache tweaks ([0e9dce4](https://github.com/immanent-tech/foragd/commit/0e9dce412c75f262204fa00c1ed0903aadfbe331))
* **service:** :zap: generate categories for subscriptions using cached data ([ac69b8c](https://github.com/immanent-tech/foragd/commit/ac69b8c0b67ce40c4aebf63273a2bbf3e3f44ec1))

## [0.136.1](https://github.com/immanent-tech/foragd/compare/v0.136.0...v0.136.1) (2026-05-12)


### Bug Fixes

* **assets:** :bug: use png for shortcut and icon images in manifest (webp support is flaky for these uses) ([15eb9ac](https://github.com/immanent-tech/foragd/commit/15eb9ac30e65a52489a6cdab29b6d3e8ac9bb200))
* **resend:** :bug: clean up error logic for batch sending ([e655698](https://github.com/immanent-tech/foragd/commit/e6556983d24a938e48f07a7a53d8663735386d72))


### Performance Improvements

* **middlewares:** :zap: increase max request per second for rate-limiting ([740e211](https://github.com/immanent-tech/foragd/commit/740e211a058f5314c721438d4106a093b8a9bab0))

## [0.136.0](https://github.com/immanent-tech/foragd/compare/v0.135.0...v0.136.0) (2026-05-12)


### Features

* **middlewares:** :sparkles: set some default values for various CSP directives ([a0ee7cb](https://github.com/immanent-tech/foragd/commit/a0ee7cb97ad3f8311da068b0859f3ba5f0d6613f))

## [0.135.0](https://github.com/immanent-tech/foragd/compare/v0.134.0...v0.135.0) (2026-05-12)


### Features

* **assets:** :sparkles: more pwa manifest improvements ([1e7b5f2](https://github.com/immanent-tech/foragd/commit/1e7b5f2f8fff47043a8803bc94b235b0e38fab4b))


### Bug Fixes

* **assets:** :bug: add missing webp version of logo ([8c29295](https://github.com/immanent-tech/foragd/commit/8c292959e714ac9f750c48539cfa77379006f739))
* **elastic:** :bug: ignore since clause when timestamp is zero value ([19fb115](https://github.com/immanent-tech/foragd/commit/19fb11590f3609483117b8f4db5aafd9f101b792))
* **search:** :bug: don't set a publishededwithin filter for searches when all time is requested ([fcdc23e](https://github.com/immanent-tech/foragd/commit/fcdc23e6c5004017a94aa1411def8b13e81ee843))
* **service:** :bug: when getting latest items for subscriptions, actually filter by view status ([2f4ef1f](https://github.com/immanent-tech/foragd/commit/2f4ef1f5d7c9252a3dea12d0da5d7eef30eb8217))


### Performance Improvements

* **service:** :zap: stored cached values by reference ([c5bc39a](https://github.com/immanent-tech/foragd/commit/c5bc39a50426311b182ae96691987ebaf4479d9c))

## [0.134.0](https://github.com/immanent-tech/foragd/compare/v0.133.0...v0.134.0) (2026-05-12)


### Features

* **assets:** :sparkles: improved pwa manifest ([24e300c](https://github.com/immanent-tech/foragd/commit/24e300c975005617f6fbc5bdd9f2297b026596d4))
* **handlers:** :sparkles: support a query parameter for url on add subscription for protocol handling in web app ([7dcf34e](https://github.com/immanent-tech/foragd/commit/7dcf34eb8b35e1310a501a16272d5641784f191f))
* **models:** :recycle: perform additional cleanups on email subscription articles ([6cd39d5](https://github.com/immanent-tech/foragd/commit/6cd39d5e2f80a05e0763c23f02adadc566c36641))
* **server:** :goal_net: log server panics to GCP ([c5e4e04](https://github.com/immanent-tech/foragd/commit/c5e4e04e0da43c9b1f6224097f18d399d5a99cee))


### Bug Fixes

* **elastic:** :bug: fix logging ([13afb06](https://github.com/immanent-tech/foragd/commit/13afb06714b0269fc930a58d9739deb25f562712))

## [0.133.0](https://github.com/immanent-tech/foragd/compare/v0.132.2...v0.133.0) (2026-05-11)


### Features

* **templates/subscriptions:** :sparkles: add ability to easily find and add a subscription to a youtube channel ([7170bb8](https://github.com/immanent-tech/foragd/commit/7170bb88fe9b04abaf4c0cb009d575f894fb703a))

## [0.132.2](https://github.com/immanent-tech/foragd/compare/v0.132.1...v0.132.2) (2026-05-10)


### Bug Fixes

* **service:** :bug: fix loading logic for users/feeds ([c3a4434](https://github.com/immanent-tech/foragd/commit/c3a4434555e8593ad424985e9c4420a3387b87aa))

## [0.132.1](https://github.com/immanent-tech/foragd/compare/v0.132.0...v0.132.1) (2026-05-10)


### Bug Fixes

* **middlewares:** :lock: set sane defaults for CSP directives that don't fallback to default-src ([94131ce](https://github.com/immanent-tech/foragd/commit/94131ce60346dfd3dba9962a8f5a6c2c4e833ccb))
* **pkg/formats/html:** :bug: fix scoping of variables ([3698764](https://github.com/immanent-tech/foragd/commit/369876482fc76f58bc42cace8641b918a91c78e2))

## [0.132.0](https://github.com/immanent-tech/foragd/compare/v0.131.3...v0.132.0) (2026-05-10)


### Features

* **templates/subscriptions:** :sparkles: add loading indicator while subscribing to a feed ([ae6301f](https://github.com/immanent-tech/foragd/commit/ae6301f9a4bc39760a3ee4194adbec68a49ee0b4))


### Bug Fixes

* :bug: fix scoping of variables ([2eef84c](https://github.com/immanent-tech/foragd/commit/2eef84c177d1a02978134eb4a84733098d8153b2))


### Performance Improvements

* **handlers:** :zap: adjust sitemap Cache-Control header ([39e5dd3](https://github.com/immanent-tech/foragd/commit/39e5dd37b86482e3619501486e852ae91935f85d))
* **service:** :zap: implement caching of feeds ([391c3a6](https://github.com/immanent-tech/foragd/commit/391c3a65638526fb1aed4ff249439cb0db2ea91d))

## [0.131.3](https://github.com/immanent-tech/foragd/compare/v0.131.2...v0.131.3) (2026-05-09)


### Bug Fixes

* :bug: go back to referring page if clicking back button on report issue form ([147ed6d](https://github.com/immanent-tech/foragd/commit/147ed6df53dfc4778070f97d15bace3014dc97bd))
* **models:** :bug: trim any trailing slash from image urls ([ba8245d](https://github.com/immanent-tech/foragd/commit/ba8245db9e54bbf28307db61b6ae211952325364))
* **templates/articles:** :bug: fix number of article images to load with high priority on page ([7c2a581](https://github.com/immanent-tech/foragd/commit/7c2a5812bf0adfa952befb7c29a7432c3e65bbba))

## [0.131.2](https://github.com/immanent-tech/foragd/compare/v0.131.1...v0.131.2) (2026-05-09)


### Bug Fixes

* **service:** :bug: recalculate dynamic info whenever subscriptions are retrieved ([bf5dc8e](https://github.com/immanent-tech/foragd/commit/bf5dc8eccafeb1bfdc33b50fef82ba56bc7e762c))
* **templates/articles:** :bug: reset display state of button if fetching remote content failed ([8f1e96b](https://github.com/immanent-tech/foragd/commit/8f1e96bfdd18e5f66416b8fbb69099759af22594))


### Performance Improvements

* :zap: cache items when generating articles for re-use ([222a5ad](https://github.com/immanent-tech/foragd/commit/222a5ada04b585542cf3698ef95d0b5affd37f92))

## [0.131.1](https://github.com/immanent-tech/foragd/compare/v0.131.0...v0.131.1) (2026-05-08)


### Reverts

* **scripts:** :fire: stop using masonry layout, too buggy ([9d32595](https://github.com/immanent-tech/foragd/commit/9d32595f4dbd0fa1bb9e9f51c69c7ef8bcab287a))

## [0.131.0](https://github.com/immanent-tech/foragd/compare/v0.130.1...v0.131.0) (2026-05-08)


### Features

* **templates/home:** :sparkles: show more latest subscriptions on home page ([f970ef6](https://github.com/immanent-tech/foragd/commit/f970ef68bf42e5d28653beffe4606bf5b8662bf0))


### Bug Fixes

* **handlers:** :bug: fix logic for feed suggestions when adding a new feed ([f27a98c](https://github.com/immanent-tech/foragd/commit/f27a98c709bf4a007f715c32c049606d72b6abc4))
* **models:** :bug: use source_url when generating new feed_id ([a98d56b](https://github.com/immanent-tech/foragd/commit/a98d56bc9f3e55583dcc93777c53252c975f0cbc))
* **service:** :bug: fix handling pagination/count when filtering subscriptions ([5511487](https://github.com/immanent-tech/foragd/commit/55114870abd177cda3310d4f83981bbba77b42db))
* **styles:** :bug: apply css to remove tracker images for both web and email article content ([9441625](https://github.com/immanent-tech/foragd/commit/9441625de3ca912878db91bf3e8df063b1aeaebf))
* **templates/articles:** :bug: don't use transitions for swapping between feed and remote article content ([2a5df4e](https://github.com/immanent-tech/foragd/commit/2a5df4eba4356341520374d66f48020d11930503))

## [0.130.1](https://github.com/immanent-tech/foragd/compare/v0.130.0...v0.130.1) (2026-05-08)


### Bug Fixes

* **elastic:** :bug: fix elastic logging ([16aaee6](https://github.com/immanent-tech/foragd/commit/16aaee66a95f3081ea85f24de68db3504257dd02))

## [0.130.0](https://github.com/immanent-tech/foragd/compare/v0.129.0...v0.130.0) (2026-05-08)


### Features

* **elastic:** :sparkles: add match_phrase query ([daff4ed](https://github.com/immanent-tech/foragd/commit/daff4edce4f6d9e1467e4018521d57ca321d665a))
* **search:** :sparkles: search tweaks ([1eac14f](https://github.com/immanent-tech/foragd/commit/1eac14f4653a1fe745ad0b261dab9ec5bf7e9742))


### Bug Fixes

* **handlers/subscriptions:** :bug: fix suggestions logic for adding new feed subscription ([77eabd7](https://github.com/immanent-tech/foragd/commit/77eabd7be9db16e5e40ce804dfab99f34745fe0e))

## [0.129.0](https://github.com/immanent-tech/foragd/compare/v0.128.2...v0.129.0) (2026-05-07)


### Features

* :sparkles: add a contact us page ([3d0ae3a](https://github.com/immanent-tech/foragd/commit/3d0ae3a1bee8fd576aab9d6a3d04b2d537f3d00c))
* **templates/subscriptions:** :sparkles: improved add subscription form ([0938903](https://github.com/immanent-tech/foragd/commit/09389032a43e683e62c8b24ea579304869e652ae))
* **templates/viewer:** :sparkles: improved viewer page ([0da880d](https://github.com/immanent-tech/foragd/commit/0da880d1e5d054e40688e8858565cb1dfe29add3))


### Bug Fixes

* **assets:** :bug: add sitemap to robots.txt ([c4c8142](https://github.com/immanent-tech/foragd/commit/c4c814289fee62edbf1a04c73e85bee9beffc457))
* **templates/contact:** :bug: fix placement of cloudflare turnstile widget ([1847b43](https://github.com/immanent-tech/foragd/commit/1847b43536cd5f767220765e6d2882e94f3913bf))

## [0.128.2](https://github.com/immanent-tech/foragd/compare/v0.128.1...v0.128.2) (2026-05-07)


### Bug Fixes

* **handlers:** :bug: fix scroll on updates ([8ef92e0](https://github.com/immanent-tech/foragd/commit/8ef92e0ec9ae5775b6700515171e77f485d160e6))
* **scripts:** :bug: ignore updating progress bar when there is no progress bar ([f754932](https://github.com/immanent-tech/foragd/commit/f754932b5d42b4eb7eacb682229632295d01d1eb))


### Performance Improvements

* **handlers:** :zap: tweak matching feeds for suggestions when adding new feed subscriptions ([72cc205](https://github.com/immanent-tech/foragd/commit/72cc2051777f66f5b7dc1f7c3f61afbc0bc9f197))

## [0.128.1](https://github.com/immanent-tech/foragd/compare/v0.128.0...v0.128.1) (2026-05-07)


### Bug Fixes

* **service:** :bug: after updating subscriptions, recalculate dynamic info before updating in cache ([3f5745e](https://github.com/immanent-tech/foragd/commit/3f5745e1f88d077d8782a9bd39764a9dabe74c15))

## [0.128.0](https://github.com/immanent-tech/foragd/compare/v0.127.1...v0.128.0) (2026-05-07)


### Features

* :recycle: subscriptions handling caching and rewrite ([f004fd1](https://github.com/immanent-tech/foragd/commit/f004fd166c00fcf69d7cee1af7b8a09cd28b9d30))
* **elastic:** :sparkles: add wildcard query support ([1542f4d](https://github.com/immanent-tech/foragd/commit/1542f4dc8ababc4dc73647699c597290c62cbcf1))
* **scripts:** :sparkles: always halt event bubbling for HandleReadStatus behaviour ([d28d2fc](https://github.com/immanent-tech/foragd/commit/d28d2fcf5d79827ca5d611ebf8d316bfeb855a71))


### Bug Fixes

* **auth0:** :bug: avoid redirecting updates/paginate routes. instead, redirect to base page ([941c4ed](https://github.com/immanent-tech/foragd/commit/941c4ed013a4802bef49ef2755642b48aa0abba4))
* **templates:** :bug: reading progress indicator working in firefox as well as chrome ([4104847](https://github.com/immanent-tech/foragd/commit/4104847ad01541d3c34783f8eeabe614b41093f4))
* **templates/articles:** :bug: fix loading indicator when fetching remote article content ([4cc834a](https://github.com/immanent-tech/foragd/commit/4cc834afdbca47e488a4218f73be3645fba45499))

## [0.127.1](https://github.com/immanent-tech/foragd/compare/v0.127.0...v0.127.1) (2026-05-05)


### Bug Fixes

* **styles:** :bug: only trigger show/hide loading changes when parent element has htmx-request class ([be10dba](https://github.com/immanent-tech/foragd/commit/be10dba0ef2e27fdc4150cbaaa138b240f243cfc))


### Performance Improvements

* **server:** :zap: add explicit caching header for hyperscript files ([5cf3be2](https://github.com/immanent-tech/foragd/commit/5cf3be278d92adbc7d11882f3563962c117be516))
* **session:** :zap: implement a caching on top of session store ([387d0d3](https://github.com/immanent-tech/foragd/commit/387d0d38b77e5bd99ce56225ea549c56d5f30581))

## [0.127.0](https://github.com/immanent-tech/foragd/compare/v0.126.1...v0.127.0) (2026-05-05)


### Features

* **templates:** :sparkles: remove cards when toggling read status when not viewing all (both read and unread) cards ([5d86add](https://github.com/immanent-tech/foragd/commit/5d86add3085bc9ad7b18fda0ea7374f0699d82ff))


### Bug Fixes

* **handlers/articles:** :bug: don't do any swap when auto-marking when viewing article content ([50bb5d1](https://github.com/immanent-tech/foragd/commit/50bb5d11118d2d67b701c437a10ed440154df130))


### Performance Improvements

* :recycle: split code into service package and start implementing in-memory caches ([ca67339](https://github.com/immanent-tech/foragd/commit/ca67339ef886106fdff9e69014f355105df0f45b))

## [0.126.1](https://github.com/immanent-tech/foragd/compare/v0.126.0...v0.126.1) (2026-05-04)


### Bug Fixes

* **handlers:** :bug: use referer header value (if any) for creating issues ([f44b641](https://github.com/immanent-tech/foragd/commit/f44b641f78c9cef9b7d8eb9336b78dd4aa124187))
* **handlers:** :fire: remove extractor references ([767bcea](https://github.com/immanent-tech/foragd/commit/767bceaeeef95eeb7314ff10695d3800f4ac6591))
* **templates:** :bug: fix mailto parts encoding ([ca37e23](https://github.com/immanent-tech/foragd/commit/ca37e234b78336874b85938353f19ff0eefc01cb))
* **templates/sidebar:** :bug: fix sizing of loading icons in sidebar ([52d2ecc](https://github.com/immanent-tech/foragd/commit/52d2eccce40f451a18529bffbff0155d34a9596d))

## [0.126.0](https://github.com/immanent-tech/foragd/compare/v0.125.2...v0.126.0) (2026-05-04)


### Features

* :recycle: issue reporting now done through email ([2cabf12](https://github.com/immanent-tech/foragd/commit/2cabf12efc77fdb4a5dce1ba35e87496bfb90ada))


### Bug Fixes

* **templates/articles:** :lipstick: fix loading indicator on buttons on article content pages ([c302dab](https://github.com/immanent-tech/foragd/commit/c302dab80f7dbb3504cab03262b110e78aea82ca))

## [0.125.2](https://github.com/immanent-tech/foragd/compare/v0.125.1...v0.125.2) (2026-05-03)


### Bug Fixes

* **templates/articles:** :lipstick: fix reading time placement ([8ba7d69](https://github.com/immanent-tech/foragd/commit/8ba7d694b1beb934ed00944cf66238a5511cfc3d))

## [0.125.1](https://github.com/immanent-tech/foragd/compare/v0.125.0...v0.125.1) (2026-05-03)


### Bug Fixes

* **scripts:** :bug: add a client-side event to catch unauthorized htmx requests and redirect to login ([f3f1817](https://github.com/immanent-tech/foragd/commit/f3f18177603b9ad55849cae256064c840226c6d8))
* **scripts:** :bug: adjust steps for resetting masonry grid after htmx swaps ([8c1232b](https://github.com/immanent-tech/foragd/commit/8c1232b85c27f74913fbd6f2296b61f71a29d57d))
* **templates/articles:** :bug: fix fetching of full article content from remote with hyperscript ([abdd4c6](https://github.com/immanent-tech/foragd/commit/abdd4c6297ac641fdce3e5db559881e1cb53f5c7))
* **templates/articles:** :bug: fix layout of byline and actions on article content pages ([120191b](https://github.com/immanent-tech/foragd/commit/120191bc69c3d0ca500a158c773963e89adcc58d))

## [0.125.0](https://github.com/immanent-tech/foragd/compare/v0.124.1...v0.125.0) (2026-05-03)


### Features

* **reverseproxy:** :sparkles: add an experimental worker using the cloudflare browser API ([f7e56b4](https://github.com/immanent-tech/foragd/commit/f7e56b42b963ff07767ca02514d0960b7d0d1178))


### Bug Fixes

* :bug: improved get article remote content handling ([f56a822](https://github.com/immanent-tech/foragd/commit/f56a8220f3f66e49a6809b287de046be88526314))
* **middlewares:** :bug: redirect htmx requests where auth has expired appropriately ([10e6124](https://github.com/immanent-tech/foragd/commit/10e61244300933c79de3f94ffa0f9766fb89973e))
* **templates/articles:** :bug: fix view remote article button ([0aa036b](https://github.com/immanent-tech/foragd/commit/0aa036bb5d5ac6a7fff0f680b795ca829acdfa08))
* **templates/articles:** :lipstick: make sure actions are right-aligned ([d1b3ec3](https://github.com/immanent-tech/foragd/commit/d1b3ec39cfa3a0f7315b48768253ca01318d5573))
* **templates/partials:** :bug: don't set loading="lazy" if fetchpriority="high" on proxied images ([d08c727](https://github.com/immanent-tech/foragd/commit/d08c7271cf3f14c3481b7826bc7c291bc86b0899))


### Performance Improvements

* **templates/articles:** :zap: set an appropriate loading attribute based on the image index ([87d72bb](https://github.com/immanent-tech/foragd/commit/87d72bb57677408a4294059e0fcf0865432e4b25))

## [0.124.1](https://github.com/immanent-tech/foragd/compare/v0.124.0...v0.124.1) (2026-05-02)


### Bug Fixes

* **scripts:** :bug: try to fix masonry grid updates ([559be9a](https://github.com/immanent-tech/foragd/commit/559be9a68e8364c70d13889b454418bd88874a2c))

## [0.124.0](https://github.com/immanent-tech/foragd/compare/v0.123.0...v0.124.0) (2026-05-02)


### Features

* **styles:** :sparkles: add a loading indicator hyperscript behaviour ([11e6940](https://github.com/immanent-tech/foragd/commit/11e6940a73d2923b5ce652b5fad3b281b49f0555))


### Bug Fixes

* **templates/articles:** :bug: fix article content buttons layout ([b0c9b0b](https://github.com/immanent-tech/foragd/commit/b0c9b0b1cc6c845e1b3054a844bca6a8baed42bc))

## [0.123.0](https://github.com/immanent-tech/foragd/compare/v0.122.0...v0.123.0) (2026-05-02)


### Features

* :sparkles: proxy requests for full article content via the user's browser instead of a backend service ([c800826](https://github.com/immanent-tech/foragd/commit/c8008266bd771ad472e873840b5df68d2f7ffdf0))
* **templates:** :lipstick: more style improvements ([4f65c0e](https://github.com/immanent-tech/foragd/commit/4f65c0e5b957ea7c5d9f8387cc2aa5ad7d819667))
* **templates:** :lipstick: show dock/sidebar indicator even when on article content pages ([49520ca](https://github.com/immanent-tech/foragd/commit/49520cac71215a73604417dee3f8858cded8dba6))


### Bug Fixes

* **config:** :bug: set the version variable based on environment variable if found ([1ff98d3](https://github.com/immanent-tech/foragd/commit/1ff98d3ad76d9acb188ba923eeba3cabb6a1930f))
* **models:** :lock: sanitise parts of item with potential unsafe content ([84f0004](https://github.com/immanent-tech/foragd/commit/84f0004c9098e88d62635b808053ae560c8c6b74))
* **styles:** :bug: ensure appropriate font smoothing is applied on osx ([f8705c6](https://github.com/immanent-tech/foragd/commit/f8705c618ebb3663a278654acad3c215d253692e))
* **styles:** :lipstick: prose article display tweaks ([0bf3553](https://github.com/immanent-tech/foragd/commit/0bf3553022bfac58995571fee78439a2d5945660))

## [0.122.0](https://github.com/immanent-tech/foragd/compare/v0.121.3...v0.122.0) (2026-05-01)


### Features

* **email:** :sparkles: add inactive account deletion notice email template ([fba6c45](https://github.com/immanent-tech/foragd/commit/fba6c454632abac22e49483fc5d4400ef4dff821))
* **email:** :sparkles: simplified header and footer components ([db1b0ba](https://github.com/immanent-tech/foragd/commit/db1b0baffd2cae51eab7ad3314c5136f409996c6))
* **models:** :sparkles: add pending_deletion metadata flag to user objects ([08d4511](https://github.com/immanent-tech/foragd/commit/08d4511e7ac5e85566e9716cc5c311f8e351f4a5))
* **resend:** :sparkles: improved functional options ([645d4a1](https://github.com/immanent-tech/foragd/commit/645d4a1566daf7efc6ee72ea12e0e818169c5d0a))
* **styles:** :sparkles: add cormorant-garamond font ([7c657a9](https://github.com/immanent-tech/foragd/commit/7c657a961a489d4f13519116868bd4c593fda6b7))
* **styles:** :sparkles: add nunito font for headings/display ([58916cc](https://github.com/immanent-tech/foragd/commit/58916cc49c78d8f8f97772698d07624f74c07029))
* **styles:** :wrench: apply font features to display font ([d87103c](https://github.com/immanent-tech/foragd/commit/d87103c439dca421873966d04cfe3adfc9c7548f))
* **templates:** :lipstick: lots of style improvements ([2ea57a9](https://github.com/immanent-tech/foragd/commit/2ea57a9fe1fd62d2f7238e512540283a935ea54b))
* **templates:** :lipstick: use display font on more page headings ([7efb624](https://github.com/immanent-tech/foragd/commit/7efb6242894e33b1eacc71641f1ebfd6ec9bf6e9))
* **templates/landing:** :lipstick: use display font on landing ([7f61460](https://github.com/immanent-tech/foragd/commit/7f6146093a83ce2c19c758ce3509f1f81a32ff2b))
* **templates/partials:** :lipstick: nicer animation on scroll to top button ([2067ec3](https://github.com/immanent-tech/foragd/commit/2067ec35fae26b41c4d39e34b5acec2f4f02e354))


### Bug Fixes

* **client:** :bug: handle non-absolute URLs when searching for images ([8f9600f](https://github.com/immanent-tech/foragd/commit/8f9600f735d7783247dfd3eeb175c92ecfe89943))
* **client:** :bug: handle non-absolute URLs when searching for images ([def5238](https://github.com/immanent-tech/foragd/commit/def523830a6c1a807382fad82dbad440ed9a65d1))
* **email:** :lipstick: consistent header and footer usage across email templates ([b4a5d1d](https://github.com/immanent-tech/foragd/commit/b4a5d1dd85f9da39c06f3caf6eb283ee9aaf4c60))
* **models:** :ambulance: metadata fields should be boolean not binary! ([05447ab](https://github.com/immanent-tech/foragd/commit/05447abe0ade061ae2239b06c798db2cc7c2df69))
* **styles:** :bug: fix font weight for tex gyre pagella font ([bf0a5a5](https://github.com/immanent-tech/foragd/commit/bf0a5a5c59bf5ffbc0f3220c9cfb65e7b33e734c))

## [0.121.3](https://github.com/immanent-tech/foragd/compare/v0.121.2...v0.121.3) (2026-04-30)


### Bug Fixes

* :bug: properly return new index aliases ([998055d](https://github.com/immanent-tech/foragd/commit/998055daa677e3e92009d25c1955ae44fd9e70f3))
* **session:** :bug: init config (noop if already done) when creating session store ([1c23336](https://github.com/immanent-tech/foragd/commit/1c2333608513072188eb58c592f6a98550b2898a))

## [0.121.2](https://github.com/immanent-tech/foragd/compare/v0.121.1...v0.121.2) (2026-04-30)


### Bug Fixes

* **schema:** :bug: fix checking elasticsearch error ([ebc804a](https://github.com/immanent-tech/foragd/commit/ebc804a981d90f50689cb2658d138785c8758310))
* **session:** :bug: don't duplicate index names ([31c45b7](https://github.com/immanent-tech/foragd/commit/31c45b7859088546d37b83b54c8022c489d12a63))

## [0.121.1](https://github.com/immanent-tech/foragd/compare/v0.121.0...v0.121.1) (2026-04-30)


### Bug Fixes

* **config:** :bug: ensure environment has a sane default set ([a84f141](https://github.com/immanent-tech/foragd/commit/a84f141ce7d3287ee2b9e7639152ebd38b416b74))
* **handlers/articles:** :bug: fix route for refresh in updates toast ([5c572f9](https://github.com/immanent-tech/foragd/commit/5c572f9fd91564e4674318973d282d988b5469f6))

## [0.121.0](https://github.com/immanent-tech/foragd/compare/v0.120.0...v0.121.0) (2026-04-30)


### Features

* **elastic:** :sparkles: improved ExtractAggregations method that can recursively extract ([4236e75](https://github.com/immanent-tech/foragd/commit/4236e757c82b1b46a83fe5f1948495ead6efd476))
* **templates/articles:** :sparkles: show reading time on longer articles ([3265394](https://github.com/immanent-tech/foragd/commit/3265394e6fe3343c441c5d03d5e8442080867540))

## [0.120.0](https://github.com/immanent-tech/foragd/compare/v0.119.0...v0.120.0) (2026-04-29)


### Features

* **elastic:** :sparkles: improved and more flexible match and multimatch query options ([522fb17](https://github.com/immanent-tech/foragd/commit/522fb17c959223e4eaea704423fb8b2bc1497bb7))
* **elastic:** :sparkles: more flexible query option settings ([8696c57](https://github.com/immanent-tech/foragd/commit/8696c57be6632994fc85e58b5762a261f429efa6))
* **elastic:** :sparkles: support setting boost and query name for terms query ([7255591](https://github.com/immanent-tech/foragd/commit/72555910a4ffec4060d2d05ea9f172e691beb1ef))
* **handlers:** :sparkles: boost results from favorite subscriptions for latest articles on home page and search suggestions/results ([a107e5a](https://github.com/immanent-tech/foragd/commit/a107e5a5da54978254068ff8f145ab3f4e00081d))


### Performance Improvements

* **elastic:** :recycle: improved requests with common functional options ([5eac493](https://github.com/immanent-tech/foragd/commit/5eac49365d2bc7f6a5358b7acd80984a40c8ada6))

## [0.119.0](https://github.com/immanent-tech/foragd/compare/v0.118.0...v0.119.0) (2026-04-28)


### Features

* **templates:** :sparkles: add function to generate json-ld script from marshaled json data ([ef5869c](https://github.com/immanent-tech/foragd/commit/ef5869cf38f8ea07dd2cb1622461827b0ab0d368))
* **templates/posts:** :sparkles: improved posts page ([b37bdd1](https://github.com/immanent-tech/foragd/commit/b37bdd1fe0bcc1e07b313b8da74118c4a6025bfa))

## [0.118.0](https://github.com/immanent-tech/foragd/compare/v0.117.0...v0.118.0) (2026-04-28)


### Features

* **templates/posts:** :sparkles: post layout improvements ([d77eb22](https://github.com/immanent-tech/foragd/commit/d77eb22755ab657827b87327e686c2eb7b9125c6))


### Bug Fixes

* **models:** :bug: add markdown file changes ([9af815b](https://github.com/immanent-tech/foragd/commit/9af815be74a6112faa3a45876bfa1b22d95ff6a8))
* **posts:** :bug: fix id for json-ld faq data ([5c6988f](https://github.com/immanent-tech/foragd/commit/5c6988f838b228c9ecb25b070f79aa53284a9b1c))
* **posts:** :bug: fix link to individual blog posts in json-ld data ([c5c60de](https://github.com/immanent-tech/foragd/commit/c5c60de31691201d3171644b90af814de2cb9a30))
* **posts:** :bug: fix url in json-ld data ([fd7e6dc](https://github.com/immanent-tech/foragd/commit/fd7e6dc9f28153c12ce33a6880643c3725bbf649))
* **templates:** :lipstick: fix spacing for filter controls on desktop and larger screens ([daa2bab](https://github.com/immanent-tech/foragd/commit/daa2bab03d8058ae70b905f3311aec705c028439))
* **templates/landing:** :bug: fix missing alt tags on blog post images ([92bf7b7](https://github.com/immanent-tech/foragd/commit/92bf7b7a2225de146f5f7408755b11fd51594d5a))
* **templates/landing:** :fire: fix missing pricing id ([57dbb41](https://github.com/immanent-tech/foragd/commit/57dbb4182aa88c334057282e6079a8929d39b7a6))
* **templates/posts:** :bug: fix author itemprop metadata format ([981c061](https://github.com/immanent-tech/foragd/commit/981c061e4d6d072248f24405b557bf71ea5d8749))
* **templates/posts:** :bug: post page fixes ([0d2c40f](https://github.com/immanent-tech/foragd/commit/0d2c40f31d3a944dca2f860445c7a193dc75811d))

## [0.117.0](https://github.com/immanent-tech/foragd/compare/v0.116.0...v0.117.0) (2026-04-27)


### Features

* **templates/comparison:** :sparkles: improved and expanded Feedly comparison ([694e1c7](https://github.com/immanent-tech/foragd/commit/694e1c78b257d3231913fd20ce2338a2a87e7f79))
* **templates/comparison:** :sparkles: improved and expanded inoreader comparision ([170b389](https://github.com/immanent-tech/foragd/commit/170b389a3c4cf1d3cb3c282d7eb2a8d85accd2d5))
* **templates/landing:** :sparkles: landing page improvements ([54663f4](https://github.com/immanent-tech/foragd/commit/54663f4e2e2c57b7bee53d8c03d4a2f77dbbe35c))


### Bug Fixes

* :sparkles: add logic for new compare pages ([8b7abc1](https://github.com/immanent-tech/foragd/commit/8b7abc1ae0f1960a4de1c15a60c5c703fb46ca07))
* **handlers:** :bug: fix canonical links ([a4f801a](https://github.com/immanent-tech/foragd/commit/a4f801a2e4a8f6cb44ff41790d814b3bbcf77976))
* **styles:** :wheelchair: fix color contrast in greenhouse theme ([be25765](https://github.com/immanent-tech/foragd/commit/be25765363ce3b38964dee0dd02d1ad25315d355))
* **templates:** :bug: fix features link ([86438d0](https://github.com/immanent-tech/foragd/commit/86438d0670d7c9005c588c0f269a6863b9913051))
* **templates:** :lipstick: make sure content container has appropriate background color ([2573aac](https://github.com/immanent-tech/foragd/commit/2573aac6e8115aa202dfb0f2daacdd46bece13dc))
* **templates/partials:** :lipstick: fix text color on back to top button ([442b724](https://github.com/immanent-tech/foragd/commit/442b7241ac3fe66cd45f68655f07b94867491c1e))

## [0.116.0](https://github.com/immanent-tech/foragd/compare/v0.115.2...v0.116.0) (2026-04-27)


### Features

* **templates:** :sparkles: seo and geo optimisation (esp. on landing page) ([f5027aa](https://github.com/immanent-tech/foragd/commit/f5027aa2c2062ddc1d3f48fab875f107a8cf0acd))
* **templates:** :sparkles: support adding "slots" in templates easily ([114db49](https://github.com/immanent-tech/foragd/commit/114db499dbd6a0cbf4171a20f251f42a01b45d8b))

## [0.115.2](https://github.com/immanent-tech/foragd/compare/v0.115.1...v0.115.2) (2026-04-27)


### Bug Fixes

* **handlers/home:** :bug: don't run aggregations if there are no feed subscriptions unread ([ac6e900](https://github.com/immanent-tech/foragd/commit/ac6e900b2109c4aa0db7efdffc3c7b106ccf92f4))
* **models:** :bug: fix add subscription logic ([77cfee0](https://github.com/immanent-tech/foragd/commit/77cfee09a527bca3a1bdb5ecb6f3cc7c28e7d95f))
* **templates:** :bug: fix rss feed link to use https ([d88feb4](https://github.com/immanent-tech/foragd/commit/d88feb4355f2651b4924b8c80ec663f9ef9dfdd9))
* **templates/landing:** :recycle: landing page fixes ([d271f3f](https://github.com/immanent-tech/foragd/commit/d271f3f39a8604e3e5c6653dc8b5cf1ce598c0ca))

## [0.115.1](https://github.com/immanent-tech/foragd/compare/v0.115.0...v0.115.1) (2026-04-26)


### Miscellaneous Chores

* release 0.115.1 ([659523d](https://github.com/immanent-tech/foragd/commit/659523d0350bb161f6a0ecdef0f8a8f2095e89f8))

## [0.115.0](https://github.com/immanent-tech/foragd/compare/v0.114.3...v0.115.0) (2026-04-26)


### Features

* **styles:** :lipstick: nice animations for popovers ([7e42f86](https://github.com/immanent-tech/foragd/commit/7e42f8638f7565a83d71eebbdd9c4bfc5cddf606))
* **templates:** :lipstick: improved list controls styling ([5cfe588](https://github.com/immanent-tech/foragd/commit/5cfe588a80d77daa064cbacc07f82ba9967d49de))


### Bug Fixes

* **templates:** :bug: correct size for favorites filter ([9b12930](https://github.com/immanent-tech/foragd/commit/9b12930fdbdfabb749d597d9e1a3a814754974e6))
* **templates:** :fire: remove duplicate class attribute ([15ba7b8](https://github.com/immanent-tech/foragd/commit/15ba7b8104d61003f1578940d0e79ca9f87d7a55))
* **templates:** :lipstick: fix a few minor style issues ([05e66dc](https://github.com/immanent-tech/foragd/commit/05e66dc87e30e6e1cae52c6f997c7cc646d70c09))
* **templates/articles:** :bug: fix reading position indicator in firefox ([3054665](https://github.com/immanent-tech/foragd/commit/30546650ae2a36b2be09e7a3e2e8f23eefe86ef9))
* **templates/home:** :bug: correct updates toast position ([4bb7040](https://github.com/immanent-tech/foragd/commit/4bb7040a28c3c09a416e91faba18e0efb64999e6))
* **templates/subscriptions:** :bug: fix closing unsubsribe modal after removal ([ac97345](https://github.com/immanent-tech/foragd/commit/ac973453682f9f2cd2822a790ee36498fd2425ee))

## [0.114.3](https://github.com/immanent-tech/foragd/compare/v0.114.2...v0.114.3) (2026-04-25)


### Bug Fixes

* :arrow_up: update go-syndication for url discovery fix ([fad70f0](https://github.com/immanent-tech/foragd/commit/fad70f0e6070a13761265254c47fa76df198389b))

## [0.114.2](https://github.com/immanent-tech/foragd/compare/v0.114.1...v0.114.2) (2026-04-25)


### Bug Fixes

* **templates/subscriptions:** :bug: fix checking for feed image when previewing feed for addition ([c1caeed](https://github.com/immanent-tech/foragd/commit/c1caeedfde9e9b6ff0994c56544fffa708c452c5))

## [0.114.1](https://github.com/immanent-tech/foragd/compare/v0.114.0...v0.114.1) (2026-04-25)


### Bug Fixes

* **models:** :bug: don't try to generate dynamic subscription info if no subscriptions were retrieved ([cb5c42b](https://github.com/immanent-tech/foragd/commit/cb5c42b848eb3aae1641d06e37701ac7af4b2383))

## [0.114.0](https://github.com/immanent-tech/foragd/compare/v0.113.2...v0.114.0) (2026-04-25)


### Features

* **client:** :sparkles: proxy all get main content requests through reverse proxy ([aada877](https://github.com/immanent-tech/foragd/commit/aada87722803c2cb9fec0fd5c1f11be4e512772c))


### Bug Fixes

* **cli:** :bug: fix item output formatting ([48ee01c](https://github.com/immanent-tech/foragd/commit/48ee01cd4873173655a8db6c74310b880f062733))
* **models:** :bug: better detection of item images ([eafa669](https://github.com/immanent-tech/foragd/commit/eafa66933bcdd6fffa26375157af407f998a66e6))

## [0.113.2](https://github.com/immanent-tech/foragd/compare/v0.113.1...v0.113.2) (2026-04-24)


### Bug Fixes

* :bug: fix incorrect ordering of articles when sorting by date ([afbf6e7](https://github.com/immanent-tech/foragd/commit/afbf6e7841c85f8c5084c7f28855fcd80248c0ed))

## [0.113.1](https://github.com/immanent-tech/foragd/compare/v0.113.0...v0.113.1) (2026-04-24)


### Bug Fixes

* **scheduler:** :bug: fix checking and updating feed image ([8d2bce1](https://github.com/immanent-tech/foragd/commit/8d2bce1a5fbab5e03d859c847d7ee148661575db))

## [0.113.0](https://github.com/immanent-tech/foragd/compare/v0.112.0...v0.113.0) (2026-04-24)


### Features

* :sparkles: new screenshots/screencast for add content feature ([1e1efdb](https://github.com/immanent-tech/foragd/commit/1e1efdb5b9b629c3a5b50521f0b3e2e241df2f8c))


### Bug Fixes

* **templates:** :bug: don't override scroll on pagination ([75f2832](https://github.com/immanent-tech/foragd/commit/75f2832d5bfb2166816f16875a9fcab0600e1c79))
* **templates:** :bug: when clicking the refresh button on the updates toast, scroll back to the top of the page ([3750a60](https://github.com/immanent-tech/foragd/commit/3750a600ee4df09fe345c6f600332c6b0debc14b))
* **templates/subscriptions:** :bug: ignore hitting enter key when suggestions input is focused for adding subscriptions ([c9011c9](https://github.com/immanent-tech/foragd/commit/c9011c974549157676772ee1150a9605df5eb1dc))

## [0.112.0](https://github.com/immanent-tech/foragd/compare/v0.111.0...v0.112.0) (2026-04-24)


### Features

* **blog:** :memo: add Kagi Small Web to finding feeds page ([ce68c7d](https://github.com/immanent-tech/foragd/commit/ce68c7df9112027212aa1ebab4b244a7e2caff86))
* **templates:** :sparkles: allow removing a subscription from its articles list page ([bb66669](https://github.com/immanent-tech/foragd/commit/bb66669865cfa2231607f1159c6e203b380824a6))


### Bug Fixes

* **templates/header:** :bug: fix pricing link ([5488eae](https://github.com/immanent-tech/foragd/commit/5488eae57c91cde305a5340d6347ed921f45decf))


### Performance Improvements

* **templates:** :zap: try to smooth pagination jumps ([81d9101](https://github.com/immanent-tech/foragd/commit/81d91011b80d62b7c298e48c5eecf98c6e299d36))
* **templates/home:** :zap: show less articles on home page ([2783550](https://github.com/immanent-tech/foragd/commit/278355049a2c3f02f92f55c4a47de526f8d2cfa8))

## [0.111.0](https://github.com/immanent-tech/foragd/compare/v0.110.0...v0.111.0) (2026-04-23)


### Features

* **models:** :recycle: don't sort favorites before other subscriptions ([7082ed8](https://github.com/immanent-tech/foragd/commit/7082ed8e3fbf066ca1c91f682c2614366f61c8b5))
* **templates/articles:** :sparkles: limit the number of categories shown on article cards to max 10 and hide all but the first 3 behind a show more button ([45bc7a6](https://github.com/immanent-tech/foragd/commit/45bc7a696dbdf6d1ad2890b303b9b853007bd974))


### Bug Fixes

* **auth0:** :bug: fix api form submission with resty ([5b476cc](https://github.com/immanent-tech/foragd/commit/5b476cc7b44b18800a1f396ed099cb75de034085))


### Performance Improvements

* :zap: switch back to readability for extracting article content ([8fe8ed4](https://github.com/immanent-tech/foragd/commit/8fe8ed41e5ba62dbea9d9f5ca3cc80a5670909f8))

## [0.110.0](https://github.com/immanent-tech/foragd/compare/v0.109.0...v0.110.0) (2026-04-23)


### Features

* :sparkles: new add feed subscription flow ([1ba4ea2](https://github.com/immanent-tech/foragd/commit/1ba4ea2c44348885d23c0d0cdaf4d8f67778f5b8))
* **templates/settings:** :sparkles: drag and drop avatar editing with image preview ([2cf9b3b](https://github.com/immanent-tech/foragd/commit/2cf9b3be09ab7ac2e0f04071362a392a3f5ec264))


### Bug Fixes

* **scripts:** :bug: fix updating masonry layout on htmx swaps ([68b586a](https://github.com/immanent-tech/foragd/commit/68b586aab86e04d09773eeedc5ff3efc9ab3c2e0))
* **templates:** :bug: fix scrolling smoothly with hyperscript ([2766101](https://github.com/immanent-tech/foragd/commit/2766101697ef8582c61e8775f05fc5215c39efb0))

## [0.109.0](https://github.com/immanent-tech/foragd/compare/v0.108.0...v0.109.0) (2026-04-21)


### Features

* **templates/subscriptions:** :sparkles: drag and drop opml file import support ([590970a](https://github.com/immanent-tech/foragd/commit/590970a5641046f83d671d6d7574b70a907d0549))


### Bug Fixes

* **extractor:** :bug: fix validation of token ttl setting ([335da41](https://github.com/immanent-tech/foragd/commit/335da419a44bcca9679f93405b286553c12b1549))

## [0.108.0](https://github.com/immanent-tech/foragd/compare/v0.107.0...v0.108.0) (2026-04-21)


### Features

* :sparkles: use new extractor service to fetch full article content ([78db271](https://github.com/immanent-tech/foragd/commit/78db27155fef39d67a71a8ec516916f575f8776c))
* **templates/element:** :sparkles: add additional hx-* functional option setters ([6f2a92d](https://github.com/immanent-tech/foragd/commit/6f2a92dfd21d103b2bbcf987c785e6ad9c16ca96))


### Bug Fixes

* **extractor:** :bug: extractor fixes ([e61dc74](https://github.com/immanent-tech/foragd/commit/e61dc74cfc926ad6dd821b49b260aec2b35b8779))
* **templates:** :bug: fix refresh links on updates available toasts ([a901ec9](https://github.com/immanent-tech/foragd/commit/a901ec9dcbd9d58454bb4d8f08b526cecf23367b))

## [0.107.0](https://github.com/immanent-tech/foragd/compare/v0.106.1...v0.107.0) (2026-04-20)


### Features

* **extractor:** :sparkles: add structured logging ([40eb01f](https://github.com/immanent-tech/foragd/commit/40eb01fd30570c8c7d5fea20d33ebc472a09cc67))


### Bug Fixes

* **extractor:** :wrench: for some output formats, preserve images, links and formatting ([c437407](https://github.com/immanent-tech/foragd/commit/c4374076c96dea326b296344e1d0363933102568))

## [0.106.1](https://github.com/immanent-tech/foragd/compare/v0.106.0...v0.106.1) (2026-04-20)


### Bug Fixes

* **extractor:** :bug: fix environment variables and uvicorn startup options ([84b5f0b](https://github.com/immanent-tech/foragd/commit/84b5f0b9ec96e042f4e85e008cda6ca8d932e37c))

## [0.106.0](https://github.com/immanent-tech/foragd/compare/v0.105.0...v0.106.0) (2026-04-20)


### Features

* **templates/articles:** :sparkles: add a reading progress bar at top of article content ([3ecf32d](https://github.com/immanent-tech/foragd/commit/3ecf32d12cafae55f4809010719c47930ead14eb))


### Bug Fixes

* **assets:** :wheelchair: fix contrast ratios on themes ([7e2fe98](https://github.com/immanent-tech/foragd/commit/7e2fe984925a8027e7356fdad131af134850ddc3))

## [0.105.0](https://github.com/immanent-tech/foragd/compare/v0.104.0...v0.105.0) (2026-04-20)


### Features

* :recycle: new updates handlings ([23eef27](https://github.com/immanent-tech/foragd/commit/23eef275af15c9b806fc534b87200919498c7139))
* **extractor:** :sparkles: add content extractor service based on trafilatura app ([bffdfd2](https://github.com/immanent-tech/foragd/commit/bffdfd24bf0ef92402b65bca9356bd2080afa25c))


### Bug Fixes

* **handlers:** :bug: use published date for generating site feed ([c69a00b](https://github.com/immanent-tech/foragd/commit/c69a00b5f772ba560fcdcb55bd6ff5e60cb28001))

## [0.104.0](https://github.com/immanent-tech/foragd/compare/v0.103.0...v0.104.0) (2026-04-19)


### Features

* :recycle: reworked authentication logic ([4be7b62](https://github.com/immanent-tech/foragd/commit/4be7b62e597e31459b28c10d372259091c193612))


### Bug Fixes

* **auth0:** :bug: fix error checking logic when posting requests to auth0 api ([820ecfa](https://github.com/immanent-tech/foragd/commit/820ecfaf17c6a28d207ecc6574a7fdfc7402613e))
* **cli:** :loud_sound: add additional logging of steps of `feed reset-updates` cli command ([00d1e1a](https://github.com/immanent-tech/foragd/commit/00d1e1a489102844720ce20eefbc7bd5009aae08))
* **models:** :bug: item parsing fixes ([818ab56](https://github.com/immanent-tech/foragd/commit/818ab563b2538cd3116dc2f8014cfff56321719d))

## [0.103.0](https://github.com/immanent-tech/foragd/compare/v0.102.0...v0.103.0) (2026-04-17)


### Features

* **templates/partials:** :sparkles: support small logo customisation ([7d9ae67](https://github.com/immanent-tech/foragd/commit/7d9ae67ae3fa66d5b19fe36224a5b4faa761fae2))


### Bug Fixes

* **cli:** :bug: fix validation of feed_id for reset-updates feed command ([20b9ef8](https://github.com/immanent-tech/foragd/commit/20b9ef8be4972733a5b7138a4f5bbc3f12fdcb71))
* **templates:** :bug: remove full-stop on feature headings ([40a1452](https://github.com/immanent-tech/foragd/commit/40a1452186cf9ca65bbf2f0f0a73817cb7d57765))
* **templates/landing:** :bug: fix logo resolution and color ([013f11c](https://github.com/immanent-tech/foragd/commit/013f11ccd2a9984d37b6113a3f8db0dea6ccb39c))
* **templates/landing:** :bug: update main image popover ([96f8605](https://github.com/immanent-tech/foragd/commit/96f86056ee42a7f8c18ef3f5d7a207c9534ed41f))

## [0.102.0](https://github.com/immanent-tech/foragd/compare/v0.101.2...v0.102.0) (2026-04-16)


### Features

* **cli:** :sparkles: new cli commands ([6ccab2f](https://github.com/immanent-tech/foragd/commit/6ccab2fd90e779a8727dd996edf068841bb2d64b))


### Bug Fixes

* **models:** :bug: catch and return a title placeholder for items without any title ([9851afe](https://github.com/immanent-tech/foragd/commit/9851afeb4a97e2aa88ebba3569a773b4180b84df))
* **scheduler:** :bug: switch back to using last_fetched field on feeds to determine whether an update_feed hjob needs to be created ([e5cb311](https://github.com/immanent-tech/foragd/commit/e5cb311744d262cc1581ee9d702ceaf8ab2cb4f9))

## [0.101.2](https://github.com/immanent-tech/foragd/compare/v0.101.1...v0.101.2) (2026-04-16)


### Bug Fixes

* **templates/articles:** :lipstick: article card layout fixes ([7eb8df6](https://github.com/immanent-tech/foragd/commit/7eb8df642e0d53718979bc35d07b7221d71f7db6))
* **templates/articles:** :lipstick: fix style bugs on article cards ([2501411](https://github.com/immanent-tech/foragd/commit/2501411ee4c2b089ea7854aa5629c2660b092916))
* **templates/search:** :bug: don't trigger suggestions popover *after* user hits enter key to search directly ([011be0a](https://github.com/immanent-tech/foragd/commit/011be0a0b740b1f248be79c5cdab629b3aea022a))
* **templates/settings:** :bug: fix spelling mistake for id ([d564cbf](https://github.com/immanent-tech/foragd/commit/d564cbf07b61798aa0e986d05d19a1241253c5d1))
* **templates/settings:** :bug: fix tab loading and navigation on settings page ([f52a83c](https://github.com/immanent-tech/foragd/commit/f52a83ca3b03b4423c401a195765be7db1a2d500))
* **templates/subscriptions:** :lipstick: subscription card layout fixes ([f2aae50](https://github.com/immanent-tech/foragd/commit/f2aae50bd329fead3bced6aa915601aae06dc2d1))

## [0.101.1](https://github.com/immanent-tech/foragd/compare/v0.101.0...v0.101.1) (2026-04-16)


### Bug Fixes

* **handlers:** :lock: protect against session fixation attacks ([cb3a4c9](https://github.com/immanent-tech/foragd/commit/cb3a4c9014c7219c51a9d294c9da5643015820eb))

## [0.101.0](https://github.com/immanent-tech/foragd/compare/v0.100.0...v0.101.0) (2026-04-16)


### Features

* **assets:** :sparkles: add Tex Gyre Pagella as the default serif font ([62bcba4](https://github.com/immanent-tech/foragd/commit/62bcba4ee667026d9870a2af874912fcc764beac))
* **models:** :sparkles: define an authors object to represent feed/item author data ([e41d74c](https://github.com/immanent-tech/foragd/commit/e41d74c5944bb80b8cac280104fed10604d15bbc))
* **pkg/text:** :sparkles: add new package text ([4546974](https://github.com/immanent-tech/foragd/commit/45469743df862beb3eb9e8f32b1cfbe16a1185eb))
* **styles:** :sparkles: improved font and animation settings ([a479ba3](https://github.com/immanent-tech/foragd/commit/a479ba32b2123b3394c40fd615ed2f6dfe6fe80d))
* **templates:** :recycle: load tex gyre pagella fonts ([40f9045](https://github.com/immanent-tech/foragd/commit/40f9045900ba6aa7a20e8a9851e53c1d180711e8))
* **templates:** :sparkles: add new features page and lists of features around collect, curate, consume topics ([df68213](https://github.com/immanent-tech/foragd/commit/df68213b26ea2fd67c8156b3ee15d0980d30b63d))
* **templates/articles:** :lipstick: improved article content layout ([2f3d461](https://github.com/immanent-tech/foragd/commit/2f3d4619979ddaeaccada10555cdee664c320eb5))
* **templates/settings:** :sparkles: new theme preview layout ([4fffaf9](https://github.com/immanent-tech/foragd/commit/4fffaf9a4b726606de0d63675148d9f99230b32c))
* **templates/subscriptions:** :recycle: don't clip/truncate latest/top article headlines in subscription cards ([10d605f](https://github.com/immanent-tech/foragd/commit/10d605f5f13b1ae10bde7226f42b044ccc9d6355))


### Bug Fixes

* **handlers:** :bug: correctly create opml export with website and feed urls for each entry ([590289a](https://github.com/immanent-tech/foragd/commit/590289a545ed0aea6d63f3bc342a852c74a995e6))
* **scheduler:** :bug: handle user with unset nickname ([7ab34c5](https://github.com/immanent-tech/foragd/commit/7ab34c5f180637e89acf91e3a0df26efd87def22))
* **styles:** :bug: fix scrollbar customisation ([56fdd57](https://github.com/immanent-tech/foragd/commit/56fdd57ee6e85e237dec6622fdc840c324a1a986))
* **templates/search:** :bug: correct link to add search subscription from search results actions menu ([f4ea816](https://github.com/immanent-tech/foragd/commit/f4ea816e803830e8568b88494650f4f97bb8cab3))
* **templates/subscriptions:** :bug: don't trigger card animation on clicking article links ([651ae72](https://github.com/immanent-tech/foragd/commit/651ae724ca4c3ec64bf27ed9472a7adc1596a499))


### Performance Improvements

* :zap: use a sync.Map to store latest items ([62fdfa5](https://github.com/immanent-tech/foragd/commit/62fdfa5c1fc57eca04f8138d91850ccb634bfcdf))
* **middlewares:** :zap: don't ratelimit own htmx requests ([92405a5](https://github.com/immanent-tech/foragd/commit/92405a55b978c8620e94e43e0470d0fa07bd576e))

## [0.100.0](https://github.com/immanent-tech/foragd/compare/v0.99.2...v0.100.0) (2026-04-13)


### Features

* **templates:** :sparkles: add ability to set htmx attributes on elements ([ca966c7](https://github.com/immanent-tech/foragd/commit/ca966c77a6d96eda9ad96d2d18e95b3546853fc8))
* **templates:** :sparkles: make loading more obvious on dock/sidebar buttons ([64cadeb](https://github.com/immanent-tech/foragd/commit/64cadeb6d2589d6750a488467f3ab86ed7fae7c1))


### Bug Fixes

* **email:** :bug: use png over webp in email templates for better client support ([5813718](https://github.com/immanent-tech/foragd/commit/58137183b408a649f71f37c0e0c23449921d13f3))
* **scheduler:** :bug: extend default job timeout to allow update feed jobs to run longer ([5831b5b](https://github.com/immanent-tech/foragd/commit/5831b5b0bcd77484caf32880ddac9dedec382b72))
* **templates:** :bug: fix specifying HXMethod option ([f3ee182](https://github.com/immanent-tech/foragd/commit/f3ee1826868e96c07188ecf10dea0fbe50f69eb0))
* **templates:** :lipstick: consistent category badge styling everywhere ([8418910](https://github.com/immanent-tech/foragd/commit/8418910594a6152f1ce30829f5a6d9dbe7795fc9))
* **templates:** :lipstick: don't trigger effects on card when category badge is active ([8671f73](https://github.com/immanent-tech/foragd/commit/8671f731faa7e2a8b5c798f1b8ad101acfcd3490))


### Performance Improvements

* **handlers:** :zap: send keepalive SSE packets to workaround cloudflare and SSE issues ([b9feaf5](https://github.com/immanent-tech/foragd/commit/b9feaf59d0343af1a5f607a7cfa489c8c3efad7f))

## [0.99.2](https://github.com/immanent-tech/foragd/compare/v0.99.1...v0.99.2) (2026-04-12)


### Bug Fixes

* **templates/home:** :recycle: only show articles with images on home page latest articles selection ([25b5022](https://github.com/immanent-tech/foragd/commit/25b502252f8d5f4842ea2db1e79e5e95fd1caace))

## [0.99.1](https://github.com/immanent-tech/foragd/compare/v0.99.0...v0.99.1) (2026-04-12)


### Bug Fixes

* **templates/home:** :bug: fix grid display masonry on home page ([d356a4c](https://github.com/immanent-tech/foragd/commit/d356a4ca6f7a33998cc1a12befa9d64ad3aa81ee))

## [0.99.0](https://github.com/immanent-tech/foragd/compare/v0.98.0...v0.99.0) (2026-04-12)


### Features

* **templates/partials:** :sparkles: add custom css classes for card components ([efea97e](https://github.com/immanent-tech/foragd/commit/efea97e15a34d49be2204297106a07aa39739e51))


### Bug Fixes

* **templates:** :bug: more fixes/tweaks for masonry library usage ([aec3e83](https://github.com/immanent-tech/foragd/commit/aec3e838be0b226de7fb408b1fcce4b9d72865d4))
* **templates:** :lipstick: consistent styling between dock/sidebar and less obvious borders on sidebar/header ([95cbc4d](https://github.com/immanent-tech/foragd/commit/95cbc4dc548b745b626188d310529e95f1815bbf))
* **templates:** :lipstick: don't animate title on active when clicking on actions in card footer ([9d287db](https://github.com/immanent-tech/foragd/commit/9d287db00cab0b22e12e0a9709a6e18ded263909))
* **templates:** :lipstick: fix active animation on subscription/article cards ([5896ba2](https://github.com/immanent-tech/foragd/commit/5896ba2f3c21a8ea9a670a1ca7c912441fb25625))
* **templates:** :lipstick: fix animations for hover/active on sidebar/dock buttons ([9f5d38a](https://github.com/immanent-tech/foragd/commit/9f5d38aaf624b415765ab8a8263d5753376e66ab))
* **templates:** :lipstick: fix subscription/article card animations and transitions ([86aba4e](https://github.com/immanent-tech/foragd/commit/86aba4e612a9999574c5a7489ff9b270862e130e))
* **templates/home:** :lipstick: align cards in carousel in center ([464b7ad](https://github.com/immanent-tech/foragd/commit/464b7addb93361b48d2eb2d55ac8e43d2e487cd1))
* **templates/partials:** :lipstick: adjust card border ([30b792b](https://github.com/immanent-tech/foragd/commit/30b792b4baf8daaf63c5daf81879a17054adfa1f))
* **templates/search:** :bug: fix scope of inline script variables ([e827802](https://github.com/immanent-tech/foragd/commit/e8278023f0fab0582f0e68b9b2c52d56b34fbc43))
* **templates/sidebar:** :bug: add missing end in hyperscript snippet ([6ca5d7e](https://github.com/immanent-tech/foragd/commit/6ca5d7e34c4ebf03a1fd61fd5d93d6bdf9c5467c))
* **templates/subscriptions:** :lipstick: increase font size of latest/top article links and highlight on hover ([56a2669](https://github.com/immanent-tech/foragd/commit/56a2669f82cd47d09581a9ac6de98e16b0f31ff0))


### Performance Improvements

* **models:** :zap: reduce number of cards loaded in list down further ([522f51d](https://github.com/immanent-tech/foragd/commit/522f51d4d9d0c60befb65828c719e28194be99e9))
* **models:** :zap: reduce number of subscriptions/articles requested for lists based on number visible on screen ([3ee8df1](https://github.com/immanent-tech/foragd/commit/3ee8df1ca583380ccfaa1b8491b5c707532a0b8e))
* **templates:** :zap: free up gpu resources after subscription/article card animations complete ([2e01875](https://github.com/immanent-tech/foragd/commit/2e018750d2e399eed6a54bcfc77598d6bc1c9682))

## [0.98.0](https://github.com/immanent-tech/foragd/compare/v0.97.0...v0.98.0) (2026-04-11)


### Features

* **templates:** :sparkles: improved card fit in grid layouts ([5d3c280](https://github.com/immanent-tech/foragd/commit/5d3c280e2d2409cf4a78a0c4d4ae8e4f076a63ee))
* **templates:** :sparkles: improved cards ([4f8cf4a](https://github.com/immanent-tech/foragd/commit/4f8cf4a1eb42b536b021eb15b0e33e750b40bd5d))
* **templates/articles:** :lipstick: improved article card styling ([1726191](https://github.com/immanent-tech/foragd/commit/1726191e741cc8f01e3b2bbb85321b6cae48708a))
* **templates/home:** :sparkles: use a list with dividers instead of cards for latest subscriptions on home page ([87439c8](https://github.com/immanent-tech/foragd/commit/87439c8ea4e04e6e28acf6e4921f4dce02eadb48))
* **templates/partials:** :sparkles: add a customisable card partial template ([416da31](https://github.com/immanent-tech/foragd/commit/416da312ec6811d97b78afad340ac8ece245c1ac))


### Bug Fixes

* **scripts:** :bug: improved loading of masonry library ([ce827d8](https://github.com/immanent-tech/foragd/commit/ce827d8b374dcca1dbfc6e96c06e470642140fba))
* **scripts:** :bug: tweak masonry ponyfill init ([d003ff2](https://github.com/immanent-tech/foragd/commit/d003ff2dd40b461f2f72ee3d6068422fbd638ba9))
* **templates:** :bug: add padding to grid ([d9dc235](https://github.com/immanent-tech/foragd/commit/d9dc23596826c901443e9c8e4cce747eb0d23314))

## [0.97.0](https://github.com/immanent-tech/foragd/compare/v0.96.0...v0.97.0) (2026-04-09)


### Features

* **assets:** :sparkles: dark/light theme adaptable svg favicon ([55e67f7](https://github.com/immanent-tech/foragd/commit/55e67f79717912418e93878d194b2668fa59057a))
* **templates/landing:** :sparkles: add comparison links to landing page ([0946b37](https://github.com/immanent-tech/foragd/commit/0946b373acfd50b8b39e495898654feca1c5ce90))


### Bug Fixes

* **session:** :bug: make sure session gets committed to store after getting a new refresh token ([60e2853](https://github.com/immanent-tech/foragd/commit/60e285368b0fb0e8d7cd1d59b1aa2d1b9be1421f))

## [0.96.0](https://github.com/immanent-tech/foragd/compare/v0.95.0...v0.96.0) (2026-04-09)


### Features

* **styles:** :sparkles: add new theme, "forest" ([a2f9d14](https://github.com/immanent-tech/foragd/commit/a2f9d14f00a463bfaa40501bae2e0c4f4cf28e38))
* **styles:** :sparkles: add new theme, "grove" ([bc17fb8](https://github.com/immanent-tech/foragd/commit/bc17fb8da9f9b0816de0c42d36bc19cc7af1acd4))
* **templates:** :lipstick: card styling improvements ([09ed582](https://github.com/immanent-tech/foragd/commit/09ed582ed16fd60e308b5ec94b2307246427ac79))
* **templates:** :lipstick: small logo uses primary color of theme ([5048e97](https://github.com/immanent-tech/foragd/commit/5048e97e88444accbaa5f6d38c1d41c8eab477bd))

## [0.95.0](https://github.com/immanent-tech/foragd/compare/v0.94.0...v0.95.0) (2026-04-09)


### Features

* :sparkles: add dedicated Foragd vs other services pages ([3e7434c](https://github.com/immanent-tech/foragd/commit/3e7434cdb1015e6fae95c478d3b02fedf7101ab7))


### Bug Fixes

* **models:** :bug: fix missing return on error ([1cbcc2b](https://github.com/immanent-tech/foragd/commit/1cbcc2bc216e312ac97c704502d150dbe5342254))
* **templates/comparison:** :bug: fix table header for inoreader comparison ([c78e9d0](https://github.com/immanent-tech/foragd/commit/c78e9d0727d4ddd120c9c71c1ea9f09e49f18792))
* **templates/partials:** :bug: fix thumbnail mask styling ([1ae3ec9](https://github.com/immanent-tech/foragd/commit/1ae3ec94716085a1c95067baccdd63b60bf10151))

## [0.94.0](https://github.com/immanent-tech/foragd/compare/v0.93.0...v0.94.0) (2026-04-09)


### Features

* **templates/articles:** :lipstick: make quick actions on article content look and act like buttons ([99663c4](https://github.com/immanent-tech/foragd/commit/99663c4551e05780ece6fa36a54ac1ba18cc784e))


### Bug Fixes

* **assets:** :memo: fix link to terms of services in acceptable use policy ([a876bb5](https://github.com/immanent-tech/foragd/commit/a876bb5af4321cee063e532472218dd66b53eb54))
* **assets:** :wrench: add additional blocks in robots.txt ([bed6d6e](https://github.com/immanent-tech/foragd/commit/bed6d6e598c102eab7e346375701c978800463f0))
* **scheduler:** :bug: fix unsubscribe link sent to user tips email template ([1143d30](https://github.com/immanent-tech/foragd/commit/1143d30bc55c2a51665b6c77ba50a4cfd25868f4))
* **styles:** :bug: fix email newsletter tables not respecting css container widths ([bce8fda](https://github.com/immanent-tech/foragd/commit/bce8fdae7146271825a00bdfdd41f133169fc577))
* **styles:** :lipstick: ensure embedded video iframes are displayed appropriately ([8aad4aa](https://github.com/immanent-tech/foragd/commit/8aad4aa78e891e02a29d0df6ac7daea15e6bfcfc))
* **templates/viewer:** :bug: fix subscription thumbnail size ([cdf120f](https://github.com/immanent-tech/foragd/commit/cdf120fc2eda5b0526d0b47a0ab936a4f0eda009))

## [0.93.0](https://github.com/immanent-tech/foragd/compare/v0.92.0...v0.93.0) (2026-04-08)


### Features

* **server:** :lock: add "Permissions-Policy" header ([8abcf46](https://github.com/immanent-tech/foragd/commit/8abcf462e5f05c8f5cb6eb87fa583263c4006d47))
* **templates:** :lock: add hash to external script for tailwind plus ([f25b881](https://github.com/immanent-tech/foragd/commit/f25b8815a6c13c9636c3af4e7d51ccf2c915b77f))
* **templates/footer:** :sparkles: add producthunt embed ([0faff75](https://github.com/immanent-tech/foragd/commit/0faff75fa517f63a61d7a86fd254d72054029974))
* **templates/landing:** :sparkles: landing page improvements ([8d6cd7f](https://github.com/immanent-tech/foragd/commit/8d6cd7fa0b8cf2c7fb7db5d627629c05432665ce))

## [0.92.0](https://github.com/immanent-tech/foragd/compare/v0.91.1...v0.92.0) (2026-04-08)


### Features

* **cli:** :sparkles: when deleting a user, also delete any scheduled jobs for the user ([4e47128](https://github.com/immanent-tech/foragd/commit/4e4712883f0997e7adeaae067e7ceddc082b571a))
* **server:** :sparkles: handle well-known url paths and add security.txt inside it ([3eab7a7](https://github.com/immanent-tech/foragd/commit/3eab7a745a7040eacac76edf50a1739974d4dd43))


### Bug Fixes

* **resend:** :bug: remove debugging statement ([c1c99bc](https://github.com/immanent-tech/foragd/commit/c1c99bc1d988f5646a1c5648f881ac0b37388aba))
* **search:** :bug: handle new user search requests gracefully ([0eb0cb4](https://github.com/immanent-tech/foragd/commit/0eb0cb43f032219e7bae612a7819ec9f8e2f1537))

## [0.91.1](https://github.com/immanent-tech/foragd/compare/v0.91.0...v0.91.1) (2026-04-08)


### Bug Fixes

* :bug: don't show subscription settings (not used) ([4692ede](https://github.com/immanent-tech/foragd/commit/4692ede81ffa61cd73c8915fd15093be5caef112))
* **resend:** :recycle: sending email tweaks ([e5a3b54](https://github.com/immanent-tech/foragd/commit/e5a3b540ede16fa9094a23b1b661ce04730e129b))
* **templates/home:** :bug: fix path for adding feed subscription ([3953851](https://github.com/immanent-tech/foragd/commit/39538516c8f0f54ad9c5e50044970714c0a8aa08))
* **templates/settings:** :bug: cancel button working on deactivate account modal ([94e8c90](https://github.com/immanent-tech/foragd/commit/94e8c90f01c3decb367503ec17ca5de8b0390d30))
* **templates/settings:** :bug: fix alignment of loading indicator ([325dd82](https://github.com/immanent-tech/foragd/commit/325dd820d7fb8e4ec3d258648e159737b46ac1ee))

## [0.91.0](https://github.com/immanent-tech/foragd/compare/v0.90.0...v0.91.0) (2026-04-07)


### Features

* **templates:** :sparkles: card improvements ([91f4283](https://github.com/immanent-tech/foragd/commit/91f42833db9d6c1ffbd31c4f76fe5c8488b21212))
* **templates:** :sparkles: category badge improvements ([353b231](https://github.com/immanent-tech/foragd/commit/353b23182d41ad0f26ee46771d17a89e20702db7))


### Bug Fixes

* **templates:** :bug: better feedback for filter changes on list subscriptions/articles pages ([73f1400](https://github.com/immanent-tech/foragd/commit/73f14000e7013b8612fd2becfe088abda32ec8d2))
* **templates:** :lipstick: consistent card actions styling on subscription/article cards ([854a128](https://github.com/immanent-tech/foragd/commit/854a1289d24044ac8a86239bb691e0cc444eb755))


### Performance Improvements

* **styles:** :zap: cap card entry animation delay ([4323af3](https://github.com/immanent-tech/foragd/commit/4323af3c06fec8e52da1747c5a7f9769623b0da9))
* **templates:** :zap: improve animations ([3670af8](https://github.com/immanent-tech/foragd/commit/3670af86dd78885d90e63173dcc76875670146a2))

## [0.90.0](https://github.com/immanent-tech/foragd/compare/v0.89.0...v0.90.0) (2026-04-07)


### Features

* :sparkles: automatically set an appropriate version if one isn't specified at build time ([6937c73](https://github.com/immanent-tech/foragd/commit/6937c733e2fb9baed62f0c89d2ff520d968ef7e9))
* **email:** :sparkles: email improvements ([5663b31](https://github.com/immanent-tech/foragd/commit/5663b318d62ae6cf048a846a4c0b4ffdc9eada2a))
* **templates:** :sparkles: save scroll position between page navigations ([085181e](https://github.com/immanent-tech/foragd/commit/085181e7bb54db4ea06ad223359bb0eacf76d3a2))


### Bug Fixes

* **email:** :bug: fix passing arguments to build deps ([7626498](https://github.com/immanent-tech/foragd/commit/762649822392f8d651a8145e4d76ba335d1bc7ed))

## [0.89.0](https://github.com/immanent-tech/foragd/compare/v0.88.3...v0.89.0) (2026-04-06)


### Features

* **templates/search:** :sparkles: switch back to popovers for search suggestions and advanced filtering ([b50ca01](https://github.com/immanent-tech/foragd/commit/b50ca01e7989c65a4fcbd363fba2f45b54153157))


### Bug Fixes

* **assets:** :bug: fix style settings ([d1b319f](https://github.com/immanent-tech/foragd/commit/d1b319f37e91e4c501eaf7d8644a94be93c97ca5))
* **templates/search:** :bug: hitting enter on search suggestion won't trigger global search ([d2e8d3b](https://github.com/immanent-tech/foragd/commit/d2e8d3be4847f02035b90a9ec18859025dc62470))
* **templates/search:** :sparkles: improved search suggestions navigation ([d31efe0](https://github.com/immanent-tech/foragd/commit/d31efe0134f852351e60819d5560090cb4faf2f6))

## [0.88.3](https://github.com/immanent-tech/foragd/compare/v0.88.2...v0.88.3) (2026-04-05)


### Performance Improvements

* **templates:** :zap: improve history restore state ([c273ac9](https://github.com/immanent-tech/foragd/commit/c273ac9e8bcea00dde93a8cb0a637ceb91c8f5e5))

## [0.88.2](https://github.com/immanent-tech/foragd/compare/v0.88.1...v0.88.2) (2026-04-03)


### Bug Fixes

* :bug: set theme with canonical daisyui attributes and remove flash on dark theme ([3c7391f](https://github.com/immanent-tech/foragd/commit/3c7391fa63663170e0cffaad38811143aa4721f5))


### Performance Improvements

* **models:** :zap: add a max requests per second for schema migrations involving reindex operations ([c5e5da3](https://github.com/immanent-tech/foragd/commit/c5e5da3f43e0673e7b5349e9c1cce79bebc20564))

## [0.88.1](https://github.com/immanent-tech/foragd/compare/v0.88.0...v0.88.1) (2026-04-03)


### Performance Improvements

* **handlers:** :zap: switch back to refreshing on history restore requests ([7d310a1](https://github.com/immanent-tech/foragd/commit/7d310a13398d61ee11a6c05f3509f605672917b0))

## [0.88.0](https://github.com/immanent-tech/foragd/compare/v0.87.0...v0.88.0) (2026-04-03)


### Features

* :sparkles: re-add update notifications ([c06092b](https://github.com/immanent-tech/foragd/commit/c06092b2a0d6a918e3c2aafd01199f5c0dc5be6e))


### Bug Fixes

* **handlers:** :bug: fix template fragment reference for 404 responses ([01583e6](https://github.com/immanent-tech/foragd/commit/01583e65db28e18cf1168b40363596d2475c15e1))
* **templates:** :bug: fix label for view filter ([3ec8c24](https://github.com/immanent-tech/foragd/commit/3ec8c24161b7531e5782a3d5cd5f2ef79958d618))
* **templates:** :bug: fix triggers for loading category filters ([169f75d](https://github.com/immanent-tech/foragd/commit/169f75d814c6567ddb671ebf9ae72dcff7e20429))
* **templates/search:** :bug: fix paths to add subscriptions ([fff59d1](https://github.com/immanent-tech/foragd/commit/fff59d1d0fae19d66ffdcb44cfd5e837d6c32cce))

## [0.87.0](https://github.com/immanent-tech/foragd/compare/v0.86.0...v0.87.0) (2026-04-02)


### Features

* :sparkles: add unsubscribe routes to allow users to unsubscribe from promotional emails ([47a782d](https://github.com/immanent-tech/foragd/commit/47a782d0c492b98f7d258f94ce23297917d7ac9d))
* :sparkles: allow toggling receiving promotional emails in account settings ([4454403](https://github.com/immanent-tech/foragd/commit/4454403b0559de86aca18629dd0c729320c3b4cb))
* :sparkles: improved new user flow ([304f6a3](https://github.com/immanent-tech/foragd/commit/304f6a33b86c15bcac01ecff8c670b352ae41411))
* **config:** :sparkles: add String() method for Environment type ([146ac84](https://github.com/immanent-tech/foragd/commit/146ac8432f27a979ccb8883395e770f883946759))
* **elastic:** :sparkles: add ILM option for allowing write after shrinking ([6fb88ff](https://github.com/immanent-tech/foragd/commit/6fb88ff4e1e1c4ca98fd6795e59d040262bab469))
* **elastic:** :sparkles: more bulk control ([8569171](https://github.com/immanent-tech/foragd/commit/85691718821119c87a752b16fa118cd1f0c70ee8))
* **elastic:** :sparkles: replace quote characters for better display of logs ([aa5e36e](https://github.com/immanent-tech/foragd/commit/aa5e36e312ffd2fbc9d6c5a6b2db62d5499ebd97))
* **email:** :sparkles: add unsubscribe link to new inactive user email ([591b5a7](https://github.com/immanent-tech/foragd/commit/591b5a7dd6ec950aa1f45f973b0eb2a0b74af6de))
* **email:** :sparkles: new email templates and fixes ([b2340b5](https://github.com/immanent-tech/foragd/commit/b2340b57d067bcd3a131902347ca78e5b72e8f8a))
* **models:** :sparkles: add cli commands and methods for updating ILM policies independently from indices ([e527330](https://github.com/immanent-tech/foragd/commit/e52733035fc28f5b47ac1c2a960b315990eaffa1))
* **scheduler:** :sparkles: improve feed update status logging ([e7fa524](https://github.com/immanent-tech/foragd/commit/e7fa524e67b89ed2dfba8ad85bde7c5a6e35e543))
* **templates/search:** :sparkles: return add subscription actions as search suggestions ([42d9784](https://github.com/immanent-tech/foragd/commit/42d978488053b579a47f8d8fe7a4986afbd0dcce))


### Bug Fixes

* **elastic:** :wastebasket: fix deprecated ilm action ([7540277](https://github.com/immanent-tech/foragd/commit/754027730ec6731345e9ce261cfe315c40b759cf))
* **models:** :bug: fix user subscription fields access after recent changes ([d8e0d0f](https://github.com/immanent-tech/foragd/commit/d8e0d0f8c9ecc27d75f028826e895b30747cf585))
* **models:** :bug: user subscription plan checks ([fc3a8ec](https://github.com/immanent-tech/foragd/commit/fc3a8ecd020b028596e69d969d2b3ea4cd6a4e3a))
* **scheduler:** :bug: fix generating jobdetail for already scheduler new inactive user job ([cb5f174](https://github.com/immanent-tech/foragd/commit/cb5f17476ea5cf7c09566716fccf04d8d0898caa))
* **templates:** :recycle: fix spelling and capitalization ([6dfa833](https://github.com/immanent-tech/foragd/commit/6dfa8337b3d2bf28769c2f1ea4f46e4d249e6610))


### Performance Improvements

* **resend:** :zap: support backoff for rate limits when batch sending emails ([eff644c](https://github.com/immanent-tech/foragd/commit/eff644c1f9c55002afc6ee547a8460a07aa57b74))

## [0.86.0](https://github.com/immanent-tech/foragd/compare/v0.85.0...v0.86.0) (2026-03-25)


### Features

* **email:** :sparkles: add badge component ([e2ee0b5](https://github.com/immanent-tech/foragd/commit/e2ee0b5f72030bf5398da24ce0c673b5d6ad4644))
* **email:** :sparkles: add email template management on resend backend with mage ([aa372fc](https://github.com/immanent-tech/foragd/commit/aa372fc0c4015aa8aaba49dc6f236a6843cee14f))
* **email:** :sparkles: add header component for highlighting a feature ([4c667e2](https://github.com/immanent-tech/foragd/commit/4c667e2a8d82a7605a244cbc1ef4ec558eda23be))
* **email:** :sparkles: add new inactive user email template ([b357f29](https://github.com/immanent-tech/foragd/commit/b357f29ecec81f9e13d264dc33d1d72bf3cc58a8))
* **email:** :sparkles: more email improvements ([5f7752c](https://github.com/immanent-tech/foragd/commit/5f7752c68690f6d8702c61d81a11c00a441af990))
* **resend:** :sparkles: add ability to update email templates ([47f87fe](https://github.com/immanent-tech/foragd/commit/47f87fe7532915d25d12080abb1117b17bc2e629))
* **templates:** :sparkles: add action buttons at bottom of subscription/article lists ([bc5798e](https://github.com/immanent-tech/foragd/commit/bc5798ed5fec463abecdcdb9d8495a5b476b6a33))


### Bug Fixes

* :bug: fix user deactivation ([3092a04](https://github.com/immanent-tech/foragd/commit/3092a04153104f013b621798de8e60dac1ea0abe))
* **handlers:** :bug: fix login callback logic ([b0edd3a](https://github.com/immanent-tech/foragd/commit/b0edd3a9fa90c754479b53ff2baf842a89ffa858))
* **handlers:** :bug: make sure variables are set correctly when sending new user email ([a02d631](https://github.com/immanent-tech/foragd/commit/a02d6310d599e458a98fa5650d2cd867f33b8944))
* **resend:** :bug: fix validation of to/cc/bcc when sending an email ([494ace8](https://github.com/immanent-tech/foragd/commit/494ace847b9cabc44d702d1f7f91625a0ffa3ca2))
* **templates:** :bug: pass parameter for controlling account deactivation ([ad516e0](https://github.com/immanent-tech/foragd/commit/ad516e00fa19d3f7157f93df05f5dec4afa329ea))
* **templates/subscriptions:** :recycle: when creating a search subscription, default to sorting by newest first ([0df58f1](https://github.com/immanent-tech/foragd/commit/0df58f129e6efd481328f40acc5649c346b2a053))
* update compiled templ template ([e66683b](https://github.com/immanent-tech/foragd/commit/e66683b4aaa835a8e431355a0fa2b11b812675e8))

## [0.85.0](https://github.com/immanent-tech/foragd/compare/v0.84.0...v0.85.0) (2026-03-23)


### Features

* **email:** :sparkles: use maizzle for email template generation ([72253e5](https://github.com/immanent-tech/foragd/commit/72253e5712400cb29ea8db66112602cf9d45c451))


### Bug Fixes

* :lock: make sure hitting back button after logging out does not show cached content ([739f672](https://github.com/immanent-tech/foragd/commit/739f67280194c3a63e331128f75573beca92facf))
* **assets:** :bug: fix manifest ([6cd6190](https://github.com/immanent-tech/foragd/commit/6cd61907e5827d9e3aea38a2a4d45ce997bf9464))
* **templates:** :bug: fix duplicate ids and add missing fieldset legends ([b2fc42f](https://github.com/immanent-tech/foragd/commit/b2fc42f08c3ce9c44403179a4173999f6b9d5037))
* **templates:** :bug: fix layout of external error page ([39f7718](https://github.com/immanent-tech/foragd/commit/39f771812585a7f0fd9b9e4ef9b6e5a9fa3661c2))
* **templates:** :wheelchair: add aria-label to all cards ([fe971bc](https://github.com/immanent-tech/foragd/commit/fe971bc2497919fde244497d4c33565cfd1bc58e))
* **templates/landing:** :bug: fix ids ([394849b](https://github.com/immanent-tech/foragd/commit/394849b14e7bd7b9908bf867d72928cda51755db))


### Performance Improvements

* :zap: cache tweaks ([c035a6d](https://github.com/immanent-tech/foragd/commit/c035a6d010211d814573d27a9fc518d13a626ad7))
* :zap: improve caching and history restoration ([9630505](https://github.com/immanent-tech/foragd/commit/9630505d1ff27d159a98496cee260a522b02b20c))

## [0.84.0](https://github.com/immanent-tech/foragd/compare/v0.83.1...v0.84.0) (2026-03-22)


### Features

* **templates:** :lipstick: animated icon for filter buttons ([82e3d9c](https://github.com/immanent-tech/foragd/commit/82e3d9cffdf66c9db980e430689e1f7c088ebf52))


### Bug Fixes

* **templates:** :wheelchair: add more accessibility features ([108060a](https://github.com/immanent-tech/foragd/commit/108060aeb62e126e6cb8c4ecd8877f9017c24c86))
* **templates/home:** :bug: allow triggering going to latest updated subscription on keyboard navigation ([c311de3](https://github.com/immanent-tech/foragd/commit/c311de37f02b3eb87f4be6deaa2ac6453d0cc1af))

## [0.83.1](https://github.com/immanent-tech/foragd/compare/v0.83.0...v0.83.1) (2026-03-20)


### Bug Fixes

* **templates:** :bug: correct merging of existing with new attributes in element WithAttributes option ([321c5a0](https://github.com/immanent-tech/foragd/commit/321c5a07476d456e3fa7c04e918be76abd3923b0))
* **templates:** :bug: improve navigation accessibility for cards ([a9bb379](https://github.com/immanent-tech/foragd/commit/a9bb379371d65357303923fe3861bf76579e0a38))
* **templates/header:** :bug: user avatar should be loaded eagerly ([cb3c047](https://github.com/immanent-tech/foragd/commit/cb3c047bc78fcf12303bdd8e04803f91b06c3a60))
* **templates/home:** :bug: fix width of recently updated subscriptions cards ([7a859d1](https://github.com/immanent-tech/foragd/commit/7a859d1ccac336d12068d54c5884d9eafd192fb9))
* **templates/partials:** :bug: don't override existing attributes when setting reasonable defaults for proxied images ([cc67347](https://github.com/immanent-tech/foragd/commit/cc67347c8d3aa63d6b098a24d2c89d7b3c4971bd))


### Performance Improvements

* **templates:** :zap: user `content-visibility: auto` on home page and subscription/article lists ([c1329c4](https://github.com/immanent-tech/foragd/commit/c1329c452440360e00660e04d499fb75d825f268))

## [0.83.0](https://github.com/immanent-tech/foragd/compare/v0.82.4...v0.83.0) (2026-03-20)


### Features

* **assets:** :lipstick: add default heading styles ([9c4b72f](https://github.com/immanent-tech/foragd/commit/9c4b72f6600aa2c199b04ba75015e9c4d40ba0e6))
* **models:** :sparkles: allow APIError to support additionalProperties ([6c90db2](https://github.com/immanent-tech/foragd/commit/6c90db238b91100bb445587a7bf6929e39231fb8))
* **templates/articles:** :sparkles: nicer transition when showing full content ([a809e81](https://github.com/immanent-tech/foragd/commit/a809e815ac21dcec461bd15325f600876a3b5f2a))
* **templates/favorites:** :sparkles: favorites page tweaks ([e3e3e2e](https://github.com/immanent-tech/foragd/commit/e3e3e2ea1395d15d496314f28af1e5a76c959aca))
* **templates/home:** :sparkles: improved home page ([f9bd268](https://github.com/immanent-tech/foragd/commit/f9bd268d2530418a9f9b5f0f3a9a89d5a85d510d))
* **templates/home:** :sparkles: more home page tweaks ([9b01f01](https://github.com/immanent-tech/foragd/commit/9b01f01f9268c7c24b5752559d2a244aa857874c))
* **templates/posts:** :lipstick: improved posts page layout ([adbeb1f](https://github.com/immanent-tech/foragd/commit/adbeb1f811d23968d15e68446c4b395224720283))


### Bug Fixes

* :bug: correct validation ([87f4def](https://github.com/immanent-tech/foragd/commit/87f4def1b7ade28c886aaf5f7fe4bc3b1008d59a))
* :bug: when saving display settings, also ensure current theme is saved ([684e8d2](https://github.com/immanent-tech/foragd/commit/684e8d232c691db758f7fe67050cceb4c2624914))
* **models:** :bug: if item content/description is HTML, try to sanitize it so it is well-formed ([e084418](https://github.com/immanent-tech/foragd/commit/e0844186453b5b860904eece7d3592bf546c210a))
* **templates:** :bug: allow hitting enter key after selected a suggestion trigger adding the category/subscription suggestion without submitting the entire form ([e876087](https://github.com/immanent-tech/foragd/commit/e8760879362148592cab10247c75c688dcdf03a7))
* **templates:** :bug: fix header alignment ([88be211](https://github.com/immanent-tech/foragd/commit/88be2111b215e298e9fb1f149a2c1c66386b58c2))

## [0.82.4](https://github.com/immanent-tech/foragd/compare/v0.82.3...v0.82.4) (2026-03-19)


### Bug Fixes

* **models:** :bug: more descriptive user facing error messages when importing/adding feed subscriptions ([389e2f4](https://github.com/immanent-tech/foragd/commit/389e2f40e20daf7d18a3b291e15b228e2af0883d))
* **models:** :bug: set the subscription thumbnail to the feed thumbnail when the user has not specified a thumbnail ([0fe2ca5](https://github.com/immanent-tech/foragd/commit/0fe2ca5793bee6117beb5fdc04bb171d06655c18))
* **templates:** :lipstick: add some space in import results list ([9289370](https://github.com/immanent-tech/foragd/commit/928937057978072ffb764193cd3fc3b33e48761c))
* **templates/subscriptions:** :lipstick: fix layout spacing for group subscription management ([eb9ce67](https://github.com/immanent-tech/foragd/commit/eb9ce675ab4646f658a9c124e95fa4a30e674ddc))

## [0.82.3](https://github.com/immanent-tech/foragd/compare/v0.82.2...v0.82.3) (2026-03-18)


### Bug Fixes

* **pkg/formats/html:** :bug: return a non-nil error if no main image found ([288e720](https://github.com/immanent-tech/foragd/commit/288e72034298f2c862dd40ab9621a22677e584e4))
* **templates:** :bug: remove weird transition artifacts ([76ac3b7](https://github.com/immanent-tech/foragd/commit/76ac3b737f2a96632532b5662625c7877ce38986))
* **templates:** :bug: sidebar/dock subscriptions/articles links always show unfiltered content ([05a1093](https://github.com/immanent-tech/foragd/commit/05a1093b65a2c54a17dcb5098374eb94dcf5962f))


### Performance Improvements

* **assets:** :zap: add 'allow' attribute and default to allowing access to compute-pressure api for youtube embeds ([bdd7924](https://github.com/immanent-tech/foragd/commit/bdd7924b932953c5a000c2869416a1568c233d40))

## [0.82.2](https://github.com/immanent-tech/foragd/compare/v0.82.1...v0.82.2) (2026-03-17)


### Bug Fixes

* **cli:** :bug: fix command-line handling for feed command ([14cb3e7](https://github.com/immanent-tech/foragd/commit/14cb3e7b9aa3129a9667ac8a30a7e6fa822b0082))
* **cli:** :bug: show items sorted by timestamp (newest first) ([ffc61e7](https://github.com/immanent-tech/foragd/commit/ffc61e76139432eea45885b5a3b01cf26c84fc33))
* **models:** :bug: don't return image if image url is empty string ([d1ca16a](https://github.com/immanent-tech/foragd/commit/d1ca16a96b821a8c594ec8c32e42bd6570711aa0))


### Reverts

* **models:** :rewind: don't automatically sort feed items ([ca8ff72](https://github.com/immanent-tech/foragd/commit/ca8ff72c48fe350beb06a4a935dca7e1e2b50d68))

## [0.82.1](https://github.com/immanent-tech/foragd/compare/v0.82.0...v0.82.1) (2026-03-16)


### Bug Fixes

* **scheduler:** :bug: fix check for updating feed image ([1ec9928](https://github.com/immanent-tech/foragd/commit/1ec9928eb475b2860fc40dacdc4c3e392772d53f))
* **templates:** :bug: fix label reference ([a6325f5](https://github.com/immanent-tech/foragd/commit/a6325f5105729f12c49876f4437b9d0ef28919be))
* **templates:** :wheelchair: fix aria-label ([79b1a41](https://github.com/immanent-tech/foragd/commit/79b1a41b69c2498cb1b919c1b9931f8f1eeb1614))
* **templates/articles:** :lipstick: force width of email newsletters to avoid horizontal overscroll ([7726d88](https://github.com/immanent-tech/foragd/commit/7726d88d08ce87e37685b1669093406c6d2677cb))
* **templates/viewer:** :recycle: use own feed as example ([75f39a2](https://github.com/immanent-tech/foragd/commit/75f39a2015a3e7e5084a282a23b97b974c3134dd))

## [0.82.0](https://github.com/immanent-tech/foragd/compare/v0.81.2...v0.82.0) (2026-03-15)


### Features

* **cli:** :sparkles: use strings.Builder to create feed output ([b074662](https://github.com/immanent-tech/foragd/commit/b07466252f802466558c22c7c9ca0ce5bd2a3692))
* **models:** :sparkles: feed/item image finding improvements ([b6d0f56](https://github.com/immanent-tech/foragd/commit/b6d0f566a73e75418e9ee885b44ee7a521e280db))
* **pkg/formats/html:** :sparkles: add method to find "main" image in page content using readability parser ([bf10b35](https://github.com/immanent-tech/foragd/commit/bf10b353077be94e8c7fc942a498f63f497c1ea1))
* **scheduler:** :sparkles: update additional feed data when new items have been fetched ([240f172](https://github.com/immanent-tech/foragd/commit/240f17274a8c5d4b1d114f72c42d44645c217008))


### Bug Fixes

* **middlewares:** :bug: fix refresh token failure flow ([43ae677](https://github.com/immanent-tech/foragd/commit/43ae6776a461d2017fa80690cf74628610e61f0a))
* **templates:** :bug: fix clicking subscription suggestion ([8d9f47a](https://github.com/immanent-tech/foragd/commit/8d9f47a485f1384cb7f6bb83db624f32c22b6f2b))
* **templates:** :lipstick: clamp article titles to three lines in feed viewer ([1866cee](https://github.com/immanent-tech/foragd/commit/1866cee41fad9cf363a0325380baef243c6a77b4))

## [0.81.2](https://github.com/immanent-tech/foragd/compare/v0.81.1...v0.81.2) (2026-03-13)


### Bug Fixes

* :bug: fix sizing of youtube videos on mobile ([447d682](https://github.com/immanent-tech/foragd/commit/447d682a97e48440fb7cee0b513808302c4e203d))

## [0.81.1](https://github.com/immanent-tech/foragd/compare/v0.81.0...v0.81.1) (2026-03-13)


### Reverts

* **templates:** :rewind: simple loading of bundled styles and scripts ([c6cc00b](https://github.com/immanent-tech/foragd/commit/c6cc00b0244394b285539be46d231b5e2908a1e6))

## [0.81.0](https://github.com/immanent-tech/foragd/compare/v0.80.3...v0.81.0) (2026-03-13)


### Features

* :sparkles: schema generate improvements ([4a46e3d](https://github.com/immanent-tech/foragd/commit/4a46e3ddf607a6408a04ffce368716737389e90a))


### Bug Fixes

* **cli:** :bug: fix display of commands ([ff71f8f](https://github.com/immanent-tech/foragd/commit/ff71f8f5cb99559051f7f6c8ee37f29fcb147777))
* **models:** :bug: fix tumblr feed url handling ([c61d4b3](https://github.com/immanent-tech/foragd/commit/c61d4b395aadfce7612da13a15bcfb951a189c9f))
* **models:** :bug: handle unable to find a feed image properly ([cd8842f](https://github.com/immanent-tech/foragd/commit/cd8842f34e3960e4ad6d157c72cca1f44c20f641))
* **templates:** :bug: fix generating hx-vals from filters without null values ([4efa71f](https://github.com/immanent-tech/foragd/commit/4efa71f4d007ac477bcf77278022c9b193eeba38))


### Performance Improvements

* :zap: improved loading of bundled styles and scripts ([615b95b](https://github.com/immanent-tech/foragd/commit/615b95bb68cbee388b6cf6a687c397ee1671ecd3))

## [0.80.3](https://github.com/immanent-tech/foragd/compare/v0.80.2...v0.80.3) (2026-03-12)


### Bug Fixes

* :bug: improved session save/restore on list pages ([992e229](https://github.com/immanent-tech/foragd/commit/992e22928f759b63b02c6e7b3d59f77050a421bc))
* **templates:** :wheelchair: hide decorative app icons ([73968ea](https://github.com/immanent-tech/foragd/commit/73968ea47fb1f66b1893b93c8ac6ba43ecff39ea))
* **templates/partials:** :lipstick: improved styling of headers ([86c4a41](https://github.com/immanent-tech/foragd/commit/86c4a4114545d7c42c3ebfe94e36c3577aa65b9a))


### Performance Improvements

* **templates:** :zap: add cache-busting param to scripts bundle ([3e7cc2b](https://github.com/immanent-tech/foragd/commit/3e7cc2bf007859247326036a60ee61d132cfefd1))

## [0.80.2](https://github.com/immanent-tech/foragd/compare/v0.80.1...v0.80.2) (2026-03-11)


### Bug Fixes

* **templates:** :recycle: use heading tags in footer link sections ([9b686fb](https://github.com/immanent-tech/foragd/commit/9b686fbecea7851be9fa6bbf6f2b1d59d56221b1))


### Performance Improvements

* **templates:** :zap: prefetch font ([7de4c5f](https://github.com/immanent-tech/foragd/commit/7de4c5f4f08b31658a08b620e7d7bcd749e223af))

## [0.80.1](https://github.com/immanent-tech/foragd/compare/v0.80.0...v0.80.1) (2026-03-11)


### Bug Fixes

* :recycle: do not use post requests for list filtering ([208c79e](https://github.com/immanent-tech/foragd/commit/208c79eeeb7b128174d61d522c62497088cbc3ee))
* **templates:** :bug: straight load fonts css ([5cc540e](https://github.com/immanent-tech/foragd/commit/5cc540e33cf1eaa01cd190d6ca228be84537c336))

## [0.80.0](https://github.com/immanent-tech/foragd/compare/v0.79.1...v0.80.0) (2026-03-11)


### Features

* **templates:** :sparkles: update features on landing page ([e64b382](https://github.com/immanent-tech/foragd/commit/e64b382fb3a7a9b729a2babda01e51a8024dc83c))
* **templates:** :sparkles: update screenshots on landing page ([0fdf831](https://github.com/immanent-tech/foragd/commit/0fdf8314b160a1e05a675dc85217d88f9cca362a))


### Bug Fixes

* **templates:** :bug: add loading indicator on article content pages ([ca3a58f](https://github.com/immanent-tech/foragd/commit/ca3a58fb463ee8f5d80d9cec4616aa54e0c13c4d))
* **templates:** :bug: straight up load css files to avoid flash of unstyled content ([ffe93b1](https://github.com/immanent-tech/foragd/commit/ffe93b15a57d3782445eb9b286f8ca1958d1afad))

## [0.79.1](https://github.com/immanent-tech/foragd/compare/v0.79.0...v0.79.1) (2026-03-11)


### Bug Fixes

* **templates:** :bug: quick fix/hack to refresh content on browser back navigation ([42f9aeb](https://github.com/immanent-tech/foragd/commit/42f9aeba8615bdc48dbf3ad93696cca331d360c4))

## [0.79.0](https://github.com/immanent-tech/foragd/compare/v0.78.0...v0.79.0) (2026-03-10)


### Features

* **posts:** :sparkles: add author frontmatter field for post authorship ([0bc7487](https://github.com/immanent-tech/foragd/commit/0bc7487a96656046025cff740ecdcd8d18ca160b))
* **server:** :sparkles: add posts index to sitemap ([8e5ba81](https://github.com/immanent-tech/foragd/commit/8e5ba81e65d51e7612de54917df7c54e254f3452))
* **server:** :wrench: server config includes a baseurl setting ([a1e7bee](https://github.com/immanent-tech/foragd/commit/a1e7bee893b4aab56e12dec77ad375fc3a44f16c))


### Bug Fixes

* **handlers:** :truck: update environment variable ([86dde27](https://github.com/immanent-tech/foragd/commit/86dde2737c35308c96b4b8a9992edb44492fc38c))
* **models:** :bug: validate opengraph data before using ([f9a6f87](https://github.com/immanent-tech/foragd/commit/f9a6f875ce77f6041c47388f77e81f6fa622e499))
* **models:** :fire: remove debugging output ([1020c37](https://github.com/immanent-tech/foragd/commit/1020c37e415d5af93a3f2567287511c69f4741df))
* **posts:** :recycle: don't use relative urls for links to assets and pages ([73ede37](https://github.com/immanent-tech/foragd/commit/73ede37fc9eb6d6f1f8f039f870bc5fda8d3eb9e))
* **templates:** :bug: fix preloading assets ([b5414a9](https://github.com/immanent-tech/foragd/commit/b5414a990e4a159143fedb45f8993d4305e80680))


### Performance Improvements

* **assets:** :zap: reduce post image sizes and use webp format ([42a8a58](https://github.com/immanent-tech/foragd/commit/42a8a5857af04b112841f26b14614a7d235edceb))

## [0.78.0](https://github.com/immanent-tech/foragd/compare/v0.77.0...v0.78.0) (2026-03-09)


### Features

* **templates:** :sparkles: redo filters as radio controls ([99e7cf8](https://github.com/immanent-tech/foragd/commit/99e7cf85e73c94c59d581d4f421b6724f97499fd))


### Bug Fixes

* **models:** :bug: fix error checking ([a5f4979](https://github.com/immanent-tech/foragd/commit/a5f49791092afaba452950f12a1213be9bafb5e6))

## [0.77.0](https://github.com/immanent-tech/foragd/compare/v0.76.2...v0.77.0) (2026-03-09)


### Features

* :sparkles: graceful handling of image proxy problems ([099db50](https://github.com/immanent-tech/foragd/commit/099db50eda8354bee27ad82f232eb699d01e18a8))
* **config:** :recycle: more flexible parsing env config variables ([8b77084](https://github.com/immanent-tech/foragd/commit/8b7708436df9901f66fd8b9bbab7306a47240e3c))
* **models:** :sparkles: for reddit posts, extract content out of table if necessary ([df0e38c](https://github.com/immanent-tech/foragd/commit/df0e38c23dad7a15ead138a4571a2e6e5b01cdf7))


### Bug Fixes

* **templates:** :lipstick: fix styling of action menu on article cards ([a6c6fba](https://github.com/immanent-tech/foragd/commit/a6c6fbaf436f96c9fe0c385956631eb378baf651))

## [0.76.2](https://github.com/immanent-tech/foragd/compare/v0.76.1...v0.76.2) (2026-03-06)


### Bug Fixes

* **templates:** :lipstick: fix resolution of user avatar in header ([fe26655](https://github.com/immanent-tech/foragd/commit/fe26655bea118a9da0a005f375e345c28d7b6fa3))
* **templates:** :lipstick: fix size of app icon in header ([791eb11](https://github.com/immanent-tech/foragd/commit/791eb1153f795a191d668baf00566faf7c43ce76))


### Performance Improvements

* **templates:** :zap: defer loading of css files ([de4eb8c](https://github.com/immanent-tech/foragd/commit/de4eb8c74a9b2bdcfa3ab9c339b7217d5443c89a))
* **templates:** :zap: preload fonts ([27afaf9](https://github.com/immanent-tech/foragd/commit/27afaf9b731d3b6b6e55f237a731ad6673994989))

## [0.76.1](https://github.com/immanent-tech/foragd/compare/v0.76.0...v0.76.1) (2026-03-06)


### Bug Fixes

* **scheduler:** :bug: always use feed URLs from feed object ([4883170](https://github.com/immanent-tech/foragd/commit/4883170b6fac389ea2067294b76e804379702182))

## [0.76.0](https://github.com/immanent-tech/foragd/compare/v0.75.0...v0.76.0) (2026-03-06)


### Features

* **docs:** :memo: update help documentation ([bc3a75a](https://github.com/immanent-tech/foragd/commit/bc3a75abb02ba31136222d2731556f0208bb940b))
* **templates:** :sparkles: add safe area padding ([0783103](https://github.com/immanent-tech/foragd/commit/078310306d19c030f2792295945e65b4a75fc339))

## [0.75.0](https://github.com/immanent-tech/foragd/compare/v0.74.0...v0.75.0) (2026-03-05)


### Features

* **gcp:** :sparkles: add ability to report errors in google cloud console ([53abfdd](https://github.com/immanent-tech/foragd/commit/53abfdd221dce359642ddb7f76d51c9520953ff7))
* **scheduler:** :sparkles: log failed update feed job executions to google cloud console ([b285088](https://github.com/immanent-tech/foragd/commit/b2850889edd368aad33c48c618416cb56b9d7023))


### Bug Fixes

* **templates:** :lipstick: better full-width responsive design for home page ([c5567b0](https://github.com/immanent-tech/foragd/commit/c5567b03d1bde2b2dc8f7053806583f1ea62ebd2))
* **templates:** :lipstick: fix padding/margin ([8383203](https://github.com/immanent-tech/foragd/commit/838320358d448634a3326ee03fc5c69c8742a7a7))

## [0.74.0](https://github.com/immanent-tech/foragd/compare/v0.73.1...v0.74.0) (2026-03-05)


### Features

* **templates:** :lipstick: multi-column homepage layout on desktop screens ([92efc4c](https://github.com/immanent-tech/foragd/commit/92efc4c80c53e4da500e4e92f49c9e51ff5069a7))
* **templates:** :sparkles: updated screenshots and layout on landing page ([20b4778](https://github.com/immanent-tech/foragd/commit/20b47786d12176a2e028612eec96f16df8db4851))


### Bug Fixes

* **middlewares:** :bug: fix rate limit configuration ([b50ac06](https://github.com/immanent-tech/foragd/commit/b50ac06eaaf46687811b8a351eaff7702d29f492))


### Performance Improvements

* **templates:** :zap: add dns-prefetch for auth domain ([66566ca](https://github.com/immanent-tech/foragd/commit/66566ca4aff1f8cce58a17327d2303d95743dc24))
* **templates:** :zap: embed logo svg in landing page ([ddcf51c](https://github.com/immanent-tech/foragd/commit/ddcf51c94f7a68bba193dff3907b581ec5864b3d))

## [0.73.1](https://github.com/immanent-tech/foragd/compare/v0.73.0...v0.73.1) (2026-03-04)


### Bug Fixes

* **middlewares:** :bug: also reset hash interface in hashwriter buffers ([28bb37d](https://github.com/immanent-tech/foragd/commit/28bb37d930df0295bc8cba3b042eb8eb83ae0a00))
* **templates:** :bug: fix display of newsletter email ([d4e48a5](https://github.com/immanent-tech/foragd/commit/d4e48a5e3b681c813e36b379cf46be2c51c2686d))


### Performance Improvements

* **middlewares:** :zap: reduce max requests per second on rate limiter ([8195772](https://github.com/immanent-tech/foragd/commit/81957726007ca703cdfabe6064ceb4e1c63ac5a1))
* **middlewares:** :zap: set rate limiter expire ttl ([4786ef9](https://github.com/immanent-tech/foragd/commit/4786ef9606ccf7996538c0523b914dc034be7cb5))
* **templates:** :zap: embed svg logo in template ([c2b2e0f](https://github.com/immanent-tech/foragd/commit/c2b2e0f6b2780665a9f3b8b7011a99c321f5c5b6))

## [0.73.0](https://github.com/immanent-tech/foragd/compare/v0.72.1...v0.73.0) (2026-03-04)


### Features

* **templates:** :sparkles: actual thumbnails for articles and subscriptions in suggestions ([7c98c54](https://github.com/immanent-tech/foragd/commit/7c98c5478a9ef5ffcf91d797531bf8a85f4a4b30))
* **templates:** :sparkles: add subscription suggestions for actions ([e95bad7](https://github.com/immanent-tech/foragd/commit/e95bad73db1f576eb535f87e9a57498a416a3031))


### Bug Fixes

* **templates:** :bug: fix indicator for loading category filters ([d2d0bc2](https://github.com/immanent-tech/foragd/commit/d2d0bc2b55f941bad9dcea29e9922e94dec62a3d))
* **templates:** :bug: fix unsubscribe modals blocking all page interactivity ([48d5069](https://github.com/immanent-tech/foragd/commit/48d50699cebd4d42e0cb71066f8619f8b0509569))
* **templates:** :fire: remove unnecessary hyperscript from command-pallette actions ([81585a8](https://github.com/immanent-tech/foragd/commit/81585a8c7b41156ad1f8ea24ee551b4ea18965e6))


### Performance Improvements

* **handlers:** :zap: reject requests for feed with query parameters set ([6a3070c](https://github.com/immanent-tech/foragd/commit/6a3070c15e76fb942306545d0b8ed09afb8a00b1))
* **templates:** :zap: use webp wherever possible for proxied images ([232b701](https://github.com/immanent-tech/foragd/commit/232b701ee321da18e31f1641ef190e0ca4480671))

## [0.72.1](https://github.com/immanent-tech/foragd/compare/v0.72.0...v0.72.1) (2026-03-03)


### Bug Fixes

* **middlewares:** :bug: always redirect to login on invalid token ([26c0e89](https://github.com/immanent-tech/foragd/commit/26c0e899ba9d1ccfa3b5817415231cdffbd5c20c))

## [0.72.0](https://github.com/immanent-tech/foragd/compare/v0.71.3...v0.72.0) (2026-03-03)


### Features

* **templates:** :sparkles: allow overriding the canonical link on a page ([1a2bea3](https://github.com/immanent-tech/foragd/commit/1a2bea3dfea6bece6ae0b205e91fc4bc465e8a3b))


### Bug Fixes

* :wheelchair: no animations if user prefers none ([eca6542](https://github.com/immanent-tech/foragd/commit/eca6542ae006d6b216f21b1c2be04b451c970b6d))

## [0.71.3](https://github.com/immanent-tech/foragd/compare/v0.71.2...v0.71.3) (2026-03-03)


### Bug Fixes

* **templates:** :bug: fix attribute spelling mistake ([969cd86](https://github.com/immanent-tech/foragd/commit/969cd869dbf6f6d727f9c4d5a2fc5df55c4c2afe))


### Performance Improvements

* **middlewares:** :zap: improve etag generation ([2c06ca0](https://github.com/immanent-tech/foragd/commit/2c06ca0c445e9a276e13647a52b613d321ff5c22))
* **templates:** :zap: add link preconnect for imgproxy ([438e529](https://github.com/immanent-tech/foragd/commit/438e529491de12afc6b712c10a163910571201e6))

## [0.71.2](https://github.com/immanent-tech/foragd/compare/v0.71.1...v0.71.2) (2026-03-02)


### Performance Improvements

* :zap: improved Cache-Control header for content ([df143ad](https://github.com/immanent-tech/foragd/commit/df143adcba0c3c7fe22b2a19b11400d221960038))

## [0.71.1](https://github.com/immanent-tech/foragd/compare/v0.71.0...v0.71.1) (2026-03-02)


### Bug Fixes

* **cli:** :bug: fix naming of command arguments for fetching feed ([dc7a694](https://github.com/immanent-tech/foragd/commit/dc7a694c0573aaa3a9d74e8fe053129214469cc2))
* **handlers:** :fire: remove debugging ([ea993b0](https://github.com/immanent-tech/foragd/commit/ea993b0a4c9fec3ca56c1f2a18fb841ffe1fbdf4))
* **reverseproxy:** :bug: fix spelling of envrionment variables prefix ([05c8757](https://github.com/immanent-tech/foragd/commit/05c8757cfd093779dbf51eb03b81db38e3799c01))
* **templates:** :bug: add id for notifications to correct element ([73c8742](https://github.com/immanent-tech/foragd/commit/73c874237d970be5398321992f8933a5f4f98ce3))
* **templates:** :bug: fix button alignment for copying subscription email on mobile displays ([7f791b8](https://github.com/immanent-tech/foragd/commit/7f791b8accfd83431ace8a922871fac001b25954))
* **templates:** :bug: make sure overscroll-contain is set on popovers and dialogs ([9306fb1](https://github.com/immanent-tech/foragd/commit/9306fb1d611e8c589c3ab9cee6ec91020b4ff4fb))

## [0.71.0](https://github.com/immanent-tech/foragd/compare/v0.70.1...v0.71.0) (2026-03-02)


### Features

* **cli:** :sparkles: add a command for fetching a feed ([5f9a183](https://github.com/immanent-tech/foragd/commit/5f9a1834d573e382fdabe2358c02968261b66fd8))
* **search:** :sparkles: add a separate command pallette for desktop/keyboard users ([683a2f2](https://github.com/immanent-tech/foragd/commit/683a2f2c607a585468e7ebf95d86576528ebdcc7))
* **templates:** :sparkles: improved search and suggestions ([35ce67f](https://github.com/immanent-tech/foragd/commit/35ce67f5e98128f19bd13fd833dd8d809fddda08))


### Bug Fixes

* :bug: fix parsing custom port for server and reverse proxy ([d7c649e](https://github.com/immanent-tech/foragd/commit/d7c649e045941f7ec6db892003033d66058f8af0))
* **templates:** :bug: set tabindex for search suggestions ([db9d3e2](https://github.com/immanent-tech/foragd/commit/db9d3e2841dfe87271b81151f6c5da50940a3153))

## [0.70.1](https://github.com/immanent-tech/foragd/compare/v0.70.0...v0.70.1) (2026-02-27)


### Bug Fixes

* :bug: fix generation of scripts.js ([76cbd25](https://github.com/immanent-tech/foragd/commit/76cbd25c625dab2ed31195217b3fa3396f3e2477))

## [0.70.0](https://github.com/immanent-tech/foragd/compare/v0.69.0...v0.70.0) (2026-02-27)


### Features

* :sparkles: use go-syndication open graph pkg ([a6246a8](https://github.com/immanent-tech/foragd/commit/a6246a8e1ca955ea5c5eebba5402bebda7c9e858))


### Bug Fixes

* :lock: don't generate sourcemaps in production ([69cffb9](https://github.com/immanent-tech/foragd/commit/69cffb9edc6805bff9d4db46902d80a58d19f5b0))
* **templates:** :lipstick: fix notification width on large displays ([4770837](https://github.com/immanent-tech/foragd/commit/47708371c18ee5a876a6ee27fd720debb2fd4d50))
* **templates:** commit generated files ([f821505](https://github.com/immanent-tech/foragd/commit/f8215050196a2a838b3f69f1cb7d641f5b898ed4))


### Performance Improvements

* **models:** :zap: proxy 429 responses through cloudflare ([b400d76](https://github.com/immanent-tech/foragd/commit/b400d767337cc0bce24a035572f16fd72d26a775))


### Reverts

* **templates:** :rewind: don't use idiomorph as it intereferes with some interactive elements after swapping ([dacb49f](https://github.com/immanent-tech/foragd/commit/dacb49f62e1bb72c755d38daeb825194601f47df))

## [0.69.0](https://github.com/immanent-tech/foragd/compare/v0.68.1...v0.69.0) (2026-02-26)


### Features

* **gcp:** :sparkles: gather instance id from metadata server when loading gcp config ([3e6267d](https://github.com/immanent-tech/foragd/commit/3e6267db98fd3cdbcb08b29b8bb4a7dfc87738a6))


### Bug Fixes

* **resend:** :bug: properly initialise email object ([a8422cc](https://github.com/immanent-tech/foragd/commit/a8422cc60abec1f6e90d572be56f9b877e2d4622))


### Performance Improvements

* **middlewares:** :zap: rate limit by path (and drop requests per second to compensate) ([ffd961e](https://github.com/immanent-tech/foragd/commit/ffd961ea8d3d8244c367903ae502d3b5b1c80584))

## [0.68.1](https://github.com/immanent-tech/foragd/compare/v0.68.0...v0.68.1) (2026-02-26)


### Performance Improvements

* **middlewares:** :zap: adjust rate-limiting ([5626650](https://github.com/immanent-tech/foragd/commit/5626650cd9ada8280ad4821466e632068be47945))

## [0.68.0](https://github.com/immanent-tech/foragd/compare/v0.67.0...v0.68.0) (2026-02-25)


### Features

* **client:** :sparkles: implement a http client package ([271a28c](https://github.com/immanent-tech/foragd/commit/271a28cae8f7a07b1a21a83858ac6c7a88f04ea6))


### Performance Improvements

* **server:** :zap: adjust rate-limiting ([8efff26](https://github.com/immanent-tech/foragd/commit/8efff26fef8ef3788d86ee2d2e4e633a1f9a8170))

## [0.67.0](https://github.com/immanent-tech/foragd/compare/v0.66.0...v0.67.0) (2026-02-25)


### Features

* **models:** :sparkles: add a method to detect whether string content is wrapped in a specific html element ([edeca32](https://github.com/immanent-tech/foragd/commit/edeca3244eba370f7f653a8db77ad6c580c6b690))


### Bug Fixes

* **search:** :bug: fix triggers for search requests ([4f4dbb1](https://github.com/immanent-tech/foragd/commit/4f4dbb1292001cc2224ac83a8b5a2264b77f8db9))

## [0.66.0](https://github.com/immanent-tech/foragd/compare/v0.65.0...v0.66.0) (2026-02-25)


### Features

* **elastic:** :loud_sound: log individual bulk operation failures as warning level logs ([b6eef59](https://github.com/immanent-tech/foragd/commit/b6eef5922febb4ff61f4309eb94240766bc88edb))
* **models:** :sparkles: try to discover an item image from original page if the item does not contain one ([e627d29](https://github.com/immanent-tech/foragd/commit/e627d2919033f8d2f7d80293086c93420b871286))
* **server:** :lock: add an explicit rate limit by ip on viewer url route ([40d936b](https://github.com/immanent-tech/foragd/commit/40d936b50d5cecc5b6d9656863c3bf7f49d1823a))


### Bug Fixes

* **handlers:** :bug: fix fetching remote content sending duplicate requests ([5a24925](https://github.com/immanent-tech/foragd/commit/5a24925de1b12bdc0f6875e303e6d094e7e590b6))
* **templates:** :bug: fix display and removal action for error messages ([00c20e6](https://github.com/immanent-tech/foragd/commit/00c20e6f2a1df37e41068915dba9c61b7a04e14f))
* **templates:** :bug: fix triggers for search results on global search bar ([22e4d42](https://github.com/immanent-tech/foragd/commit/22e4d427335eb0dfbbb1d2571918df4227e856f1))

## [0.65.0](https://github.com/immanent-tech/foragd/compare/v0.64.0...v0.65.0) (2026-02-24)


### Features

* **assets:** :lipstick: style scrollbar ([20af0a8](https://github.com/immanent-tech/foragd/commit/20af0a85a080357f6b8cf37769d62a5b4eeb4657))


### Bug Fixes

* **models:** :bug: make sure proxied URL is not saved to feed data ([fd2b0b5](https://github.com/immanent-tech/foragd/commit/fd2b0b5251fddf6cfab41ba908357253c5890c49))

## [0.64.0](https://github.com/immanent-tech/foragd/compare/v0.63.0...v0.64.0) (2026-02-24)


### Features

* **assets:** :bento: add placeholder image in webp format ([78c2733](https://github.com/immanent-tech/foragd/commit/78c2733c45632d2e966c7461871bd33e69a9d595))

## [0.63.0](https://github.com/immanent-tech/foragd/compare/v0.62.0...v0.63.0) (2026-02-24)


### Features

* **templates:** :fire: remove custom relative time custom element with packaged version ([1a620c4](https://github.com/immanent-tech/foragd/commit/1a620c4908f373a41fae1a56e8515285412898f0))
* **templates:** :sparkles: show search results on hitting enter key ([070406a](https://github.com/immanent-tech/foragd/commit/070406aeafc19e42d18673ddf754b64eaeca2795))


### Bug Fixes

* **templates:** :lipstick: fix favorites filter menu item ([5e158c6](https://github.com/immanent-tech/foragd/commit/5e158c6751c013d65b827662293735dd980c73c8))


### Reverts

* :rewind: switch back to hosted tailwindplus elements due to issues in production deployment ([31ac5f2](https://github.com/immanent-tech/foragd/commit/31ac5f270e3916ed982979ba8c5c2a65e224094d))

## [0.62.0](https://github.com/immanent-tech/foragd/compare/v0.61.0...v0.62.0) (2026-02-24)


### Features

* **templates:** :lipstick: improved styling of menus ([e31e10f](https://github.com/immanent-tech/foragd/commit/e31e10fd84f331eab7b41473e254f7111b27683a))


### Bug Fixes

* :bug: fix popovers causing layout shift in chrome ([4491fb6](https://github.com/immanent-tech/foragd/commit/4491fb6c6c7f2ee348742ad9fdb75bcffffb8e53))
* **assets:** :bento: make htmx aware of custom elements ([d4345c7](https://github.com/immanent-tech/foragd/commit/d4345c700281fbe4ab4add1add4a530ccf0413a6))
* **handlers:** :bug: exclude empty strings from top categories results on home page ([48caccf](https://github.com/immanent-tech/foragd/commit/48caccf2edb9fdc188d516d3696798573728131d))
* **search:** :bug: exclude empty string categories on search results page ([69e4033](https://github.com/immanent-tech/foragd/commit/69e4033b53269067a76bd789c5ff0da97417d19d))
* **templates:** :bug: fix hx-trigger for search results ([ef207c3](https://github.com/immanent-tech/foragd/commit/ef207c3e66f44edb1870cbaf4c51b6995803229b))
* **templates:** :bug: fix label elements referenced ids ([3ab16d2](https://github.com/immanent-tech/foragd/commit/3ab16d2f460fd1161eb31ac4f9862f30094e16b3))
* **templates:** :bug: hitting enter in global search should execute a global search ([12aac76](https://github.com/immanent-tech/foragd/commit/12aac7605ee8c375aaeb4128d70ea03beb289918))


### Performance Improvements

* **templates:** :zap: reduce delay in showing suggestions when searching ([243fe8d](https://github.com/immanent-tech/foragd/commit/243fe8d6272c986773dc40daa0adaab4075f884d))

## [0.61.0](https://github.com/immanent-tech/foragd/compare/v0.60.0...v0.61.0) (2026-02-23)


### Features

* :sparkles: support youtube feeds ([75be7a3](https://github.com/immanent-tech/foragd/commit/75be7a36a6020856244d58ec4cdc0cdeb3088ec2))
* :sparkles: use idiomorph htmx extension for swaps where possible ([7ef5a5b](https://github.com/immanent-tech/foragd/commit/7ef5a5bc107d7af04d94202d4f58fefffc8b959e))
* **config:** :sparkles: use a canonical format for the app id ([2db785b](https://github.com/immanent-tech/foragd/commit/2db785b995e7ce0febc3e7b735feff683c5f5519))
* **models:** :sparkles: detect and format non-HTML item descriptions as HTML ([6dba317](https://github.com/immanent-tech/foragd/commit/6dba317feb9453ef0c13d2897005d902db791447))
* **resend:** :sparkles: allow adding tags to sent emails ([4ac9f25](https://github.com/immanent-tech/foragd/commit/4ac9f25260c830c2ab8bfcd8ca512251b96f0f24))
* **resend:** :sparkles: handle recieved catch-all/admin emails ([ba9e548](https://github.com/immanent-tech/foragd/commit/ba9e548a2b820818a02c0bfbde6374bed467f1af))
* **scheduler:** :sparkles: create a job for pinging new and inactive users (account created but not logged in) ([142f61a](https://github.com/immanent-tech/foragd/commit/142f61a48d40586082ef55c3de26e289cd931356))
* **scheduler:** :sparkles: make it easier to define different jobs ([98d7dca](https://github.com/immanent-tech/foragd/commit/98d7dcacd9b63d2540a5d05c7249816caf97b9d6))


### Bug Fixes

* **handlers:** :bug: update for resend changes ([a3157a8](https://github.com/immanent-tech/foragd/commit/a3157a801b04120dae691b0a2ef4324f4574246b))
* **middlewares:** :bug: don't use broken nonce handling in CSP middleware ([032728d](https://github.com/immanent-tech/foragd/commit/032728d635fc6e554cb429950530460d7ebe7511))
* **resend:** :bug: fix config variables ([ecf016a](https://github.com/immanent-tech/foragd/commit/ecf016a092af72fe87cdd8e344d319df4e08195f))
* **scheduler:** :bug: fix generating jobdetail for scheduled jobs ([1ffd1ee](https://github.com/immanent-tech/foragd/commit/1ffd1eeca310bfc129c53db6bb1b310ffcd3b7c7))
* **scheduler:** :bug: fix job scheduling issues ([06d0fda](https://github.com/immanent-tech/foragd/commit/06d0fdab75060b5f74f983ddfa612ee632a8d9dc))
* **templates:** :bug: add missing hx-push-url attributes ([ba00d09](https://github.com/immanent-tech/foragd/commit/ba00d096d33237349d8d4fd9a52aa3defa8c546e))
* **templates:** :bug: fix remembering scroll position on using back button ([53df3c0](https://github.com/immanent-tech/foragd/commit/53df3c01a7032fdeddf0fcf320c2d208524d6222))


### Performance Improvements

* **handlers:** :zap: reverse proxy image requests for certain error responses ([a688df5](https://github.com/immanent-tech/foragd/commit/a688df500b803e01bd5b93605ea0065ce0715ff6))
* **models:** :zap: html conversion improvements ([6f147e8](https://github.com/immanent-tech/foragd/commit/6f147e829301289a6a7556c72673f76975c3e4a3))

## [0.60.0](https://github.com/immanent-tech/foragd/compare/v0.59.1...v0.60.0) (2026-02-19)


### Features

* :sparkles: new post: foragd vs inoreader vs feedly comparison ([1968cb7](https://github.com/immanent-tech/foragd/commit/1968cb7ee79f873814b3455a9e454c204ff4ec3f))
* **elastic:** :sparkles: more flexible simplequerystring clause generation with functional options ([1cd2144](https://github.com/immanent-tech/foragd/commit/1cd214482125968829536521807ab8f010925c74))
* **models:** :sparkles: track additional values for users locally ([b56ee15](https://github.com/immanent-tech/foragd/commit/b56ee157e7437beccbe82a374ac038319b38f764))
* **models:** :sparkles: track email verified for users locally ([b1a57f8](https://github.com/immanent-tech/foragd/commit/b1a57f8bd04a9bc02c7d3c7cbbeec98f7e73e8c4))
* **templates:** :sparkles: add feed viewer link to heading for non logged-in/external users ([b1cc91c](https://github.com/immanent-tech/foragd/commit/b1cc91c5edbb41bdafc6947f470a0d941fd909dd))
* **templates:** :sparkles: improved landing page ([08c0d9b](https://github.com/immanent-tech/foragd/commit/08c0d9b036bc93c9d627ef8eb150982542b48a1c))
* **templates:** :sparkles: update pricing with annual plan discount ([6a95fbd](https://github.com/immanent-tech/foragd/commit/6a95fbd384b4c57513b50e84e4abf42957474e82))


### Bug Fixes

* :bug: fix new user login quirks ([c16fc07](https://github.com/immanent-tech/foragd/commit/c16fc07cf46a45e26f422a09667f89f3ee6b5961))
* **models:** :bug: actually send updates for last_login and login_count ([35954ec](https://github.com/immanent-tech/foragd/commit/35954ecc972b9bc217d016786cfbe1627ab91774))
* **resend:** :bug: fix passing template id for sending email ([7b94c70](https://github.com/immanent-tech/foragd/commit/7b94c70d5ef782fd67063abd2abb6278475bf7a4))
* **templates:** :lipstick: fix article card images not having rounded corners ([9df70b4](https://github.com/immanent-tech/foragd/commit/9df70b4a7c3b73d8ffe71ba65829125342421c39))


### Performance Improvements

* **search:** :zap: tweak search ([7a8e22e](https://github.com/immanent-tech/foragd/commit/7a8e22e823daee2874d58fe76ad4caaf36adb7de))

## [0.59.1](https://github.com/immanent-tech/foragd/compare/v0.59.0...v0.59.1) (2026-02-18)


### Bug Fixes

* **templates:** :bug: change example url to one that actually exists ([91b6532](https://github.com/immanent-tech/foragd/commit/91b65329a7b439f3e09f991778898dc1da36108f))

## [0.59.0](https://github.com/immanent-tech/foragd/compare/v0.58.2...v0.59.0) (2026-02-18)


### Features

* **cli:** :sparkles: add scheduler sub command to list all jobs ([363f5c2](https://github.com/immanent-tech/foragd/commit/363f5c258923f9ee676342f03db5ef04ff666297))


### Bug Fixes

* **models:** :bug: fix parsing logic for well-known domains ([1736270](https://github.com/immanent-tech/foragd/commit/1736270dd0820ccc503ffd62de0428d35a0542c4))
* **reverseproxy:** :bug: fix env for reverse proxy base url ([3138863](https://github.com/immanent-tech/foragd/commit/31388638b3d7b913e5f8c88680b78e9946ae2a4b))


### Performance Improvements

* **models:** :zap: extend timeout for fetching feed details ([6695f82](https://github.com/immanent-tech/foragd/commit/6695f8247ca79e2084f70b7ed8bb926faabfcd2a))

## [0.58.2](https://github.com/immanent-tech/foragd/compare/v0.58.1...v0.58.2) (2026-02-18)


### Bug Fixes

* :bug: fix proxy error handling ([66fd715](https://github.com/immanent-tech/foragd/commit/66fd715f6bcec53ca1e4a294a80a3502159f0e17))

## [0.58.1](https://github.com/immanent-tech/foragd/compare/v0.58.0...v0.58.1) (2026-02-18)


### Bug Fixes

* :bug: allow specifying an id when fetching a feed from url to assign results to an existing feed ([a462cd8](https://github.com/immanent-tech/foragd/commit/a462cd8d280b2c903f8e375f55d623c2a831b66f))
* **scheduler:** :fire: remove debugging code ([49f8d23](https://github.com/immanent-tech/foragd/commit/49f8d23d569c7fdc74efc84d13d8acd71e066f89))

## [0.58.0](https://github.com/immanent-tech/foragd/compare/v0.57.0...v0.58.0) (2026-02-18)


### Features

* **templates:** :sparkles: add a heading on landing page ([6c7c4f5](https://github.com/immanent-tech/foragd/commit/6c7c4f5a2d9d1189a72d017da8fef55606d27e9e))
* **templates:** :sparkles: improved heading for external pages ([b9b6d59](https://github.com/immanent-tech/foragd/commit/b9b6d59aa106edade73db94f72ffe8a5f1dda44d))
* **templates:** :sparkles: use a mask for subscription thumbnails ([60b8017](https://github.com/immanent-tech/foragd/commit/60b8017d6b9cd7ba606f219b63ce1608c649142b))
* **templates:** :sparkles: use a mask on avatar images ([1708f24](https://github.com/immanent-tech/foragd/commit/1708f249034e0382c23bad19d96a4a2bcb6b61ea))


### Bug Fixes

* **assets:** :bug: fix styling of cards with image issues and ensure this style doesn't propagate elsewhere ([1b32700](https://github.com/immanent-tech/foragd/commit/1b327008f914eb7af1c0562c899222111abddab6))
* **elastic:** :bug: fix for new version ([751d3e3](https://github.com/immanent-tech/foragd/commit/751d3e36b3dba887e1e5deda7b92a003058acbdc))
* **templates:** :bug: push url for import/export to browser bar ([f6a94c5](https://github.com/immanent-tech/foragd/commit/f6a94c55b889f4b81772998c987f7581d760c769))
* **templates:** :bug: use card-img class to handle broken images on article cards ([3784298](https://github.com/immanent-tech/foragd/commit/3784298a1b05ea3a532e34eb6923197023c462f0))

## [0.57.0](https://github.com/immanent-tech/foragd/compare/v0.56.0...v0.57.0) (2026-02-17)


### Features

* :sparkles: add ability to pass a URL directly to viewer ([bd3c68c](https://github.com/immanent-tech/foragd/commit/bd3c68c2cd56dc88014954791e3ceebd7a109bd0))
* :sparkles: add link to help in footer and expose in sitemap ([23c7076](https://github.com/immanent-tech/foragd/commit/23c70767766b3e0228c5139c7b79b96326be8a55))
* :sparkles: add reverse proxy implementations for fetching restricted resources ([bb5e23b](https://github.com/immanent-tech/foragd/commit/bb5e23b8927e09b88a91fb52901d3a3342b77c14))
* **models:** :recycle: update feed fetching ([e87a0f4](https://github.com/immanent-tech/foragd/commit/e87a0f485c8a2fd3d543ae6e324933533597a83f))
* **models:** :sparkles: if fetching a feed returns a 403 response, try proxying it through cloudflare ([4c019e5](https://github.com/immanent-tech/foragd/commit/4c019e5d11798f575bd2f4385c5d7a847b20c7a6))
* **scheduler:** :sparkles: update feed job improvements ([1f06745](https://github.com/immanent-tech/foragd/commit/1f06745dbd84077585d3fb4465608dd6641743ad))
* **templates:** :sparkles: add a FAQ on the viewer page ([8713047](https://github.com/immanent-tech/foragd/commit/8713047b8cdede6b9d4a058ef058b0f4c874e592))
* **templates:** :sparkles: update text and add bookmarklet ([8dee9e5](https://github.com/immanent-tech/foragd/commit/8dee9e588d23de6406ca486853877f4e6bf79ea7))


### Bug Fixes

* **handlers:** :bug: make sure response body is closed when image proxy handler finishes running ([99f22f2](https://github.com/immanent-tech/foragd/commit/99f22f22df8a7eb190475647f8823cdc9c46e3f4))
* **models:** :bug: feed fetching fixes ([63286c1](https://github.com/immanent-tech/foragd/commit/63286c11bcafa85723a3f4eacaab843dcb2a4d2f))
* **reverseproxy:** :bug: fix sign-url script ([455bfd8](https://github.com/immanent-tech/foragd/commit/455bfd8407c5c3ed209928f98ed3561f4bcb511d))

## [0.56.0](https://github.com/immanent-tech/foragd/compare/v0.55.1...v0.56.0) (2026-02-16)


### Features

* **models:** :sparkles: support defining a path to an image to represent a file in frontmatter ([f4c8c63](https://github.com/immanent-tech/foragd/commit/f4c8c6382f96683c650ab985c9bb6d02edadb5dd))
* **templates:** :sparkles: add images for all posts ([cd75783](https://github.com/immanent-tech/foragd/commit/cd75783c2c513d63b22097ff107f33b65c34b475))

## [0.55.1](https://github.com/immanent-tech/foragd/compare/v0.55.0...v0.55.1) (2026-02-13)


### Bug Fixes

* **models:** :bug: add additional email data fields ([e680b6b](https://github.com/immanent-tech/foragd/commit/e680b6bbeda73a866615df35250351eea586f0a9))
* **models:** :bug: fix missing email_data schema definition in subscriptions ([c669b75](https://github.com/immanent-tech/foragd/commit/c669b75235a9d0cef6b45cbe467d48e6e3ebcf26))

## [0.55.0](https://github.com/immanent-tech/foragd/compare/v0.54.0...v0.55.0) (2026-02-13)


### Features

* :sparkles: suggest categories from existing subscriptions where appropriate ([57ea5d7](https://github.com/immanent-tech/foragd/commit/57ea5d7dfe4827b709335ff5fda6fa712a11b1f9))
* **templates:** :sparkles: add structured data to posts ([88ded20](https://github.com/immanent-tech/foragd/commit/88ded20322b4befc42bb91fe3d51e2122fbe1b81))

## [0.54.0](https://github.com/immanent-tech/foragd/compare/v0.53.0...v0.54.0) (2026-02-12)


### Features

* :sparkles: add article filters for group subscriptions ([4e1e0cb](https://github.com/immanent-tech/foragd/commit/4e1e0cbb678b80b47baf41f1c4f0a8b4f603fee5))
* :sparkles: support adding an additional query when filtering articles for a subscription ([5383e28](https://github.com/immanent-tech/foragd/commit/5383e283f2895b966578f6ee890d78710e173641))
* :sparkles: viewer service now parses url and can handle finding feeds on well-known domains ([eeddfcb](https://github.com/immanent-tech/foragd/commit/eeddfcb6cbba3c1b4d84a2f34198433c30a96121))
* **elastic:** :sparkles: allow specifying fuzziness for match-based query options ([3803224](https://github.com/immanent-tech/foragd/commit/3803224dc274f231b828fb746361f2c671bf6114))
* **models:** :sparkles: add ability to exclude certain subscriptions when returning results ([d6b37e0](https://github.com/immanent-tech/foragd/commit/d6b37e01c8b8de265c9b2418754e5dda86869edd))
* **models:** :sparkles: split out a method to parse and create feed urls for well-known sites ([e8b8d24](https://github.com/immanent-tech/foragd/commit/e8b8d249cc73a474faf6944aa56fc795f77a4959))
* **posts:** :sparkles: add post about managing content overload ([65bfdac](https://github.com/immanent-tech/foragd/commit/65bfdacc94c82931c613f049186141dc530df828))
* **templates:** :sparkles: add a &lt;noscript&gt; body tag ([6b36e48](https://github.com/immanent-tech/foragd/commit/6b36e4899827c8ff68d474df3c0e0afa367b3383))
* **templates:** :sparkles: toggle control for showing just favorites/all subscriptions ([e365d01](https://github.com/immanent-tech/foragd/commit/e365d011ed1ee1a10ce57025b7b4209afb5282ce))


### Bug Fixes

* :bug: don't suggest categories that a subscription already has when editing ([a468c59](https://github.com/immanent-tech/foragd/commit/a468c595f1cbd1bdc6bce0666c77b32ec10f0c7d))
* :bug: fix adding a search subscription from scratch ([8b983f4](https://github.com/immanent-tech/foragd/commit/8b983f4fc4864650917af8d8a6a15aa655f1398d))
* :bug: fix adding feed subscription from scratch ([d6b9769](https://github.com/immanent-tech/foragd/commit/d6b9769b41b4ac18cf8f70e86c82733b6ecec0da))
* :bug: fix adding group subscriptions ([e98f364](https://github.com/immanent-tech/foragd/commit/e98f36402ad8e1d1afc9ffb3379a2a62f51ec797))
* :bug: fix changing categories on a feed subscription ([a9fd46e](https://github.com/immanent-tech/foragd/commit/a9fd46efcbcbaa16aeb8917f760e60d093f24321))
* :bug: streamline group/search subscription creation ([e5a6978](https://github.com/immanent-tech/foragd/commit/e5a6978ec258b745693acfd4fd4716e6c9652e62))
* :rotating_light: remove some potential sources of nil pointer references ([ab66156](https://github.com/immanent-tech/foragd/commit/ab66156604d6fbc943305b781a68dbaafc895be1))
* **middlewares:** :bug: properly set the expiry time of the oauth token after refresh ([03d375b](https://github.com/immanent-tech/foragd/commit/03d375ba8a14f17010500d364845bab9b6336b53))
* **search:** :bug: fix adding/removing subscription filters for searching ([0102a34](https://github.com/immanent-tech/foragd/commit/0102a34582e24368b6c125cad81aff9246b3e18d))
* **templates:** :bug: disable category suggestions drop-down when there are no suggestions ([ef7f544](https://github.com/immanent-tech/foragd/commit/ef7f54472dbefbeaaf17e3f2513c1303521d39ac))
* **templates:** :bug: push url when selecting command in search suggestions ([3237c14](https://github.com/immanent-tech/foragd/commit/3237c145436756310a5b0efb570c52d511e2f59d))

## [0.53.0](https://github.com/immanent-tech/foragd/compare/v0.52.1...v0.53.0) (2026-02-10)


### Features

* **middlewares:** :recycle: switch to github.com/go-chi/cors for CORS middleware ([11b9e9c](https://github.com/immanent-tech/foragd/commit/11b9e9cc7709d8385e5190098d2ad5625d00b1e4))

## [0.52.1](https://github.com/immanent-tech/foragd/compare/v0.52.0...v0.52.1) (2026-02-10)


### Bug Fixes

* **handlers:** :bug: fix saving list filters ([dba3524](https://github.com/immanent-tech/foragd/commit/dba3524f84d7e21750c70f4bd72500f4e95f98c9))

## [0.52.0](https://github.com/immanent-tech/foragd/compare/v0.51.1...v0.52.0) (2026-02-10)


### Features

* **templates:** :sparkles: add solarized-dark theme ([a911c15](https://github.com/immanent-tech/foragd/commit/a911c1514bd5f915909efe4fce2f054e9304ff7d))


### Bug Fixes

* **templates:** :bug: fix image aspect ratio on article cards ([b3731b1](https://github.com/immanent-tech/foragd/commit/b3731b1f96ade9079784234099eafa94e73fb4ec))

## [0.51.1](https://github.com/immanent-tech/foragd/compare/v0.51.0...v0.51.1) (2026-02-10)


### Bug Fixes

* **server:** :bug: pass tracer to otel middleware ([547fa04](https://github.com/immanent-tech/foragd/commit/547fa0437f6be3e90bbb560e1debf4e68241a2ec))

## [0.51.0](https://github.com/immanent-tech/foragd/compare/v0.50.0...v0.51.0) (2026-02-10)


### Features

* **logging:** :technologist: add otel trace/span to logs ([b5b356c](https://github.com/immanent-tech/foragd/commit/b5b356ce67567708153f29360d80120eae7890b6))
* **models:** :sparkles: allow setting an updated_at timestamp for when a file index has been updated ([91fc3ba](https://github.com/immanent-tech/foragd/commit/91fc3ba0c3b3c34284ee18639168ae77de59e24d))
* **server:** :sparkles: generate rss feed of posts ([0ae13c2](https://github.com/immanent-tech/foragd/commit/0ae13c204964ff36cbb7ad5a5457b00841c8a9f8))
* **server:** :sparkles: renew the session data when the auth token is refreshed ([1354176](https://github.com/immanent-tech/foragd/commit/13541764a8a5b4d71159dac95fa3b20f4550c79f))
* **templates:** :sparkles: add categories of matching articles on top of search results page ([b80c06f](https://github.com/immanent-tech/foragd/commit/b80c06f55eb7be9472d265f67f2c49e4d67ad995))
* **templates:** :sparkles: add RSS autodiscovery ([0ca9ca8](https://github.com/immanent-tech/foragd/commit/0ca9ca8dede1dc4a49d05adc34d3eb085acbf382))


### Bug Fixes

* :bug: fix editing subscriptions categories ([88232da](https://github.com/immanent-tech/foragd/commit/88232da2b10ab8573b2cfdc5876c5e12b98816f4))
* **handlers:** :bug: set content-type header when serving sitemap ([4115581](https://github.com/immanent-tech/foragd/commit/41155811bb8f7f6a268459cb334ab16384c7e88c))
* **server:** :fire: disable broken updates polling for now ([8df301e](https://github.com/immanent-tech/foragd/commit/8df301ea4889b79e16957fc4e80c41c996b380a1))


### Reverts

* **templates:** :rewind: remove new transition style on view article ([bad20d2](https://github.com/immanent-tech/foragd/commit/bad20d2de620d9f5f574d22d4531b5c0828e81ac))

## [0.50.0](https://github.com/immanent-tech/foragd/compare/v0.49.0...v0.50.0) (2026-02-07)


### Features

* **templates:** :sparkles: landing page improvements ([f18ac5e](https://github.com/immanent-tech/foragd/commit/f18ac5e2b18da886ecd6fc32ee452b5098ef5f24))


### Bug Fixes

* **assets:** :bug: add transition-style package ([3e15675](https://github.com/immanent-tech/foragd/commit/3e156750be3efb161ffd9c2f8a5cf07f607e0536))
* **docs:** :memo: fix formatting in policy documents ([dddd0f0](https://github.com/immanent-tech/foragd/commit/dddd0f0719512343956ae827a091b7d86ce887e6))
* **templates:** :bug: change element on which new transition for view article is applied ([19a9a3f](https://github.com/immanent-tech/foragd/commit/19a9a3fcf18a065dc068c42680f260436b8c58be))
* **templates:** :bug: remove blocked (by CSP) inline-style usage ([f75a64f](https://github.com/immanent-tech/foragd/commit/f75a64f603a1f0f610f778706e52bbf7bed466b8))

## [0.49.0](https://github.com/immanent-tech/foragd/compare/v0.48.0...v0.49.0) (2026-02-06)


### Features

* **assets:** :sparkles: style broken images with placeholder ([fcc8743](https://github.com/immanent-tech/foragd/commit/fcc8743126830360728726dc2da3a1aeab76e284))
* **templates:** :lipstick: "wipe in" article content when viewing ([ea058a8](https://github.com/immanent-tech/foragd/commit/ea058a8eab81b14d0c8dd3e1a0fe164689c68722))
* **templates:** :sparkles: better opengraph metadata on pages ([559f0be](https://github.com/immanent-tech/foragd/commit/559f0be8095c29f487c34a27278791dbe4bb16eb))
* **templates:** :sparkles: show updated timestamp on subscription cards on home ([ad22003](https://github.com/immanent-tech/foragd/commit/ad220035fbd7346b97d3cf247bd27a0c72d3019b))


### Bug Fixes

* :bug: update go.mod ([bae2d03](https://github.com/immanent-tech/foragd/commit/bae2d031b696eed4fe5dde7e9db506dc2b916ba3))
* **assets:** :bento: match card image aspect ratio for placeholder image ([d8f1128](https://github.com/immanent-tech/foragd/commit/d8f1128c262b3a9690af84d9b00f5d60a0084a9a))

## [0.48.0](https://github.com/immanent-tech/foragd/compare/v0.47.0...v0.48.0) (2026-02-06)


### Features

* **templates:** :lipstick: improved card layouts ([0412717](https://github.com/immanent-tech/foragd/commit/04127175592ab1eb15f79214cd3db8d850ee4d24))


### Bug Fixes

* **resend:** :bug: fix error message format for received email errors ([d85e059](https://github.com/immanent-tech/foragd/commit/d85e059829de5643e64b6d802042e1e310890c96))
* **server:** :recycle: fix setup of open telemetry ([b0fc455](https://github.com/immanent-tech/foragd/commit/b0fc455477dc8d01634903404126c99e73092d8b))
* **templates:** :bug: fix view remote button ([b97090a](https://github.com/immanent-tech/foragd/commit/b97090a1df8ce17efbbc7c5628e460dae7227177))
* **templates:** :lipstick: display headings in summaries same size as other text ([f15425c](https://github.com/immanent-tech/foragd/commit/f15425cc8bea9c530b5c7fc416a7d9248a92c806))

## [0.47.0](https://github.com/immanent-tech/foragd/compare/v0.46.0...v0.47.0) (2026-02-05)


### Features

* :sparkles: allow direct beta sign-ups ([abe489a](https://github.com/immanent-tech/foragd/commit/abe489a5beed4dcf3470d80457d928e7cbd5524a))
* :sparkles: send beta welcome email on first login ([783d14d](https://github.com/immanent-tech/foragd/commit/783d14d0d1a061ac024b218bfa6adf971165e118))


### Bug Fixes

* :bug: don't update url for failed get remote content request ([c592f0b](https://github.com/immanent-tech/foragd/commit/c592f0b254803fe609a373095f4b5af09c685455))

## [0.46.0](https://github.com/immanent-tech/foragd/compare/v0.45.0...v0.46.0) (2026-02-04)


### Features

* :sparkles: implement proper pub/sub for updates handling ([8fa7200](https://github.com/immanent-tech/foragd/commit/8fa720089d8df55c0dd24aad2b2375deececfa1f))
* **assets:** :bento: add some more feeds to the enlightened feedset ([4976311](https://github.com/immanent-tech/foragd/commit/4976311f30192c8127c0bf3a9050a33036dce43e))
* **models:** :sparkles: add feed status schema for tracking last fetched status of feeds ([95a0718](https://github.com/immanent-tech/foragd/commit/95a07181f7667a6736e230de0bc1b735b519e776))
* **scheduler:** :sparkles: log status for feed on each update job run ([3ee2102](https://github.com/immanent-tech/foragd/commit/3ee2102d0ccaba0319fc7154ea87055b7f32b062))
* **server:** :sparkles: add opentelemetry monitoring ([b33bbef](https://github.com/immanent-tech/foragd/commit/b33bbef4328401336e2cc357cd79411b0f982a96))
* **templates:** :sparkles: add a few quality of life features to global search ([f57b68e](https://github.com/immanent-tech/foragd/commit/f57b68e3a5459275f9e13e7802f924b9452aed3c))


### Bug Fixes

* :recycle: use correct validation package ([a8048e4](https://github.com/immanent-tech/foragd/commit/a8048e47c6c2d90fe929b0ca03a8ac98a127328f))
* **models:** :recycle: change feed status schema to use a datastream ([37e2da0](https://github.com/immanent-tech/foragd/commit/37e2da09485a533326ec121f93a5b7ed97fdf58b))
* **templates:** :bug: disable pubsub updates polling ([832709f](https://github.com/immanent-tech/foragd/commit/832709fc2b07af25254b18f69aadb6c7b21f0478))


### Reverts

* :rewind: revert back to simpler SSE implementation for now ([e3b3f19](https://github.com/immanent-tech/foragd/commit/e3b3f190a63efe1ab1a15cb3fd1a6f30d2e6e44a))

## [0.45.0](https://github.com/immanent-tech/foragd/compare/v0.44.0...v0.45.0) (2026-02-03)


### Features

* **assets:** :bento: add png versions of logo ([9dd1138](https://github.com/immanent-tech/foragd/commit/9dd11382b60d84831f2fef781524adc53aa09e2f))
* **templates:** :sparkles: add link in footer to report security issues ([d248bdd](https://github.com/immanent-tech/foragd/commit/d248bdda0a2056ad40ddd048746dc82cb10ea74e))


### Bug Fixes

* :bug: define a method to properly decode multipart forms ([a0e5371](https://github.com/immanent-tech/foragd/commit/a0e5371f7a9f90c480a1e598beba39fb9d63ed7c))
* **templates:** :bug: fix link to help on new user home ([1d520ba](https://github.com/immanent-tech/foragd/commit/1d520ba7793c1410b92e4ae39d924f6e81d352bb))

## [0.44.0](https://github.com/immanent-tech/foragd/compare/v0.43.0...v0.44.0) (2026-02-02)


### Features

* **auth0:** :sparkles: switch all code to use v2 api sdk ([573c22e](https://github.com/immanent-tech/foragd/commit/573c22eb66d8b2c4830e0f8339cc3e832aaea649))


### Bug Fixes

* **handlers:** :bug: correct check for not found error ([1fb4d3c](https://github.com/immanent-tech/foragd/commit/1fb4d3c38911694e4f3e66cd695d456770877ab9))
* **handlers:** :bug: don't show error on no subscriptions on new user home (that is expected) ([262f78c](https://github.com/immanent-tech/foragd/commit/262f78cad8210065c309cee1fe48df30739c64e5))

## [0.43.0](https://github.com/immanent-tech/foragd/compare/v0.42.0...v0.43.0) (2026-02-02)


### Features

* **templates:** :sparkles: specific empty content templates for different pages ([d66d7a5](https://github.com/immanent-tech/foragd/commit/d66d7a595b678443fcbf6386609e17875bce2ad2))


### Performance Improvements

* **assets:** :zap: optmise screenshots for landing page ([5e98ced](https://github.com/immanent-tech/foragd/commit/5e98ced4f71ba214d719b73a03defc0bf86e3508))


### Reverts

* :rewind: switch back to using history snapshot feature ([cee0ec0](https://github.com/immanent-tech/foragd/commit/cee0ec0e41002e7e5289b12efae970d4effa3dc6))
* :rewind: switch back to using history snapshot feature ([ebae759](https://github.com/immanent-tech/foragd/commit/ebae759aaa4cb432d6bbb1548d54538923f777da))

## [0.42.0](https://github.com/immanent-tech/foragd/compare/v0.41.0...v0.42.0) (2026-02-02)


### Features

* **assets:** :sparkles: adjust icon colors and optimise size ([3a4d044](https://github.com/immanent-tech/foragd/commit/3a4d0444c308af5d92fe8e7621a7ecfe12928be3))


### Bug Fixes

* **assets:** :bug: fix small favicon color ([fcd5941](https://github.com/immanent-tech/foragd/commit/fcd594165cd1b3c9820ace63bd2b41448cddbb62))
* **handlers:** :bug: correctly return favicon when requested ([617830a](https://github.com/immanent-tech/foragd/commit/617830ac2cde6622ac4ed9c7b2413b50cde89500))

## [0.41.0](https://github.com/immanent-tech/foragd/compare/v0.40.2...v0.41.0) (2026-02-02)


### Features

* **assets:** :sparkles: add a better placeholder image ([8bdcbf7](https://github.com/immanent-tech/foragd/commit/8bdcbf79e6888fda29eca8da2be6bfef5acffdd6))
* **handlers:** :sparkles: switch from blackfriday to goldmark for markdown to HTML conversions ([c5302d4](https://github.com/immanent-tech/foragd/commit/c5302d4567dc5666de0185ace4a0da7b9f1d21f2))
* **middlewares:** :loud_sound: log some useful htmx-related values on requests ([d37a32f](https://github.com/immanent-tech/foragd/commit/d37a32f20ccdb3771d4e4563a039856b46b1a549))
* **posts:** :sparkles: add a post about finding feeds ([a157a33](https://github.com/immanent-tech/foragd/commit/a157a33b935d4f5d182c3bb081f57db103ce0be3))
* **templates:** :sparkles: abstract mailto link creation to package ([d9f0239](https://github.com/immanent-tech/foragd/commit/d9f0239d0ea9b6bbaf28db97f8b785be71a81b95))
* **templates:** :sparkles: make container flexible for internal and external page ([b58ed0c](https://github.com/immanent-tech/foragd/commit/b58ed0c9da4ab5f87eb5ce5382a7e380068b99aa))


### Bug Fixes

* :bug: fix missing dock on favorites page for full page load ([b835693](https://github.com/immanent-tech/foragd/commit/b835693f5095cc91a57fb9de24f55378ae5fa8d2))
* **assets:** :bug: fix loading of htmx variable ([4f8836c](https://github.com/immanent-tech/foragd/commit/4f8836ca91d090e98c3ab1d377860c5f05431ae8))
* **handlers:** :bug: allow unsafe html (i.e. image links) with markdown renderer ([8840ba6](https://github.com/immanent-tech/foragd/commit/8840ba66cf73bd8a057b49eea6d82a61b2a143f6))
* **middlewares:** :bug: do not generate new state for restoration on invalid token for list/search updates requests ([d48e98f](https://github.com/immanent-tech/foragd/commit/d48e98fb3d60d96eed4975d3137dfeabdcee7702))
* **models:** :bug: only suggest feed or email subscriptions as filters for searches ([b17077d](https://github.com/immanent-tech/foragd/commit/b17077d46a3f550b1a51c8a446fd5b9305932f78))
* **posts:** :bug: fix title in directory ([5de964e](https://github.com/immanent-tech/foragd/commit/5de964e421120d168c2a0d94e469f74c8c56a004))
* **posts:** :memo: shorten feeds vs social media title ([985681e](https://github.com/immanent-tech/foragd/commit/985681ed933e96b4639f78b2864760f3ab1906b6))
* **templates:** :bug: add notification and mention beta program and offer on landing ([3d39334](https://github.com/immanent-tech/foragd/commit/3d39334f547abb22c2f0514f98f8fbf846cdd66a))
* **templates:** :bug: control style and functionality fixes ([1f8768d](https://github.com/immanent-tech/foragd/commit/1f8768d23171f543b3985450e88007ad49ea17f7))
* **templates:** :bug: don't pass text as value to Heading2 template, pass as child ([0c20905](https://github.com/immanent-tech/foragd/commit/0c209050669cc96e95dd7a8a80ad3ae853c782df))
* **templates:** :bug: explicitly mention pricing is post-launch ([d405454](https://github.com/immanent-tech/foragd/commit/d405454ce7ad2fbfdf411abf254b9c2d2578a3e6))
* **templates:** :bug: use templ.KV for defining extra classes on proxied image ([4367ed1](https://github.com/immanent-tech/foragd/commit/4367ed122b51923f91e890313c67c7f0a94f050d))
* **templates:** :lipstick: add padding to bottom of containers ([cf69996](https://github.com/immanent-tech/foragd/commit/cf6999600e7b26c51d4fcff5b0a79479a40a4526))
* **templates:** :lipstick: even size subscription cards ([00706ea](https://github.com/immanent-tech/foragd/commit/00706ea39679297255535e006d4f8332a98cfb8a))
* **templates:** :lipstick: header should be fixed not sticky ([f5f8362](https://github.com/immanent-tech/foragd/commit/f5f836247cf819a743344115c29f0c400d86475b))
* **templates:** :lipstick: improved footer positioning ([69b80b4](https://github.com/immanent-tech/foragd/commit/69b80b46b93b49df1241ce0b9c884417a8c3a032))


### Performance Improvements

* :zap: force refresh on back button every time ([b2d6ee5](https://github.com/immanent-tech/foragd/commit/b2d6ee58aaa93e9906dbc3cff389793d49264416))

## [0.40.2](https://github.com/immanent-tech/foragd/compare/v0.40.1...v0.40.2) (2026-01-31)


### Bug Fixes

* **scheduler:** :bug: get new feeds job fixes ([d80026a](https://github.com/immanent-tech/foragd/commit/d80026afc5001669400230a53c1cc55437ed3f62))
* **server:** :bug: set user-agent string (same as scheduler) for underlying go-syndication fetcher/parser ([ee21e47](https://github.com/immanent-tech/foragd/commit/ee21e477e8b54f3a36e987993d90e4438b8ca749))

## [0.40.1](https://github.com/immanent-tech/foragd/compare/v0.40.0...v0.40.1) (2026-01-31)


### Bug Fixes

* **docs:** :bug: fix spelling and formatting ([d728818](https://github.com/immanent-tech/foragd/commit/d728818dee0839333a623dbcb649fae3a5421a84))
* **posts:** :bug: improve title wording ([d6c0cbe](https://github.com/immanent-tech/foragd/commit/d6c0cbe8f2581117c96b02382974dd2b3147e507))

## [0.40.0](https://github.com/immanent-tech/foragd/compare/v0.39.0...v0.40.0) (2026-01-31)


### Features

* **elastic:** :sparkles: add a "Before" query option to query docs with a date field "before" (i.e. older) than a given timestamp ([3a2a28b](https://github.com/immanent-tech/foragd/commit/3a2a28b2b7e954d86ca533c80d432d7562843b09))
* **scheduler:** :sparkles: add job to remove expired sessions from the session index ([524240c](https://github.com/immanent-tech/foragd/commit/524240c6f807a6b4f8019b2157753b82417fac2a))
* **templates:** :lipstick: improve home styling ([28cfd72](https://github.com/immanent-tech/foragd/commit/28cfd72e349f5ffc8245cc7bdb5e8efa51bb9169))
* **templates:** :recycle: switch to pure tailwind css slideshow for latest articles on mobile displays on home ([b7b6f6c](https://github.com/immanent-tech/foragd/commit/b7b6f6c2e58938cec6d41620c3a9b2d5c291bc91))
* **templates:** :sparkles: add partial templates for styling headers and paragraphs ([32186b7](https://github.com/immanent-tech/foragd/commit/32186b76aa93d31285e73bbdace8b62ddf56c825))
* **templates:** :sparkles: add partial templates for various containers used on content pages ([b130a27](https://github.com/immanent-tech/foragd/commit/b130a27ecaf2e35abaebd30b13aa06a4e8f6bbc0))


### Bug Fixes

* **templates:** :bug: fix article actions menu not opening in slideshow on home page ([71743b5](https://github.com/immanent-tech/foragd/commit/71743b54d37614bbe3329cbfb1235d214d9fd243))
* **templates:** :lipstick: dock adjustments ([1cdd2be](https://github.com/immanent-tech/foragd/commit/1cdd2be81f57c0c4c50d8a48d8e43654dd3a6402))


### Performance Improvements

* **server:** :zap: improve push critical assets middleware and usage ([80bdb7e](https://github.com/immanent-tech/foragd/commit/80bdb7e301c4f772e1fc9f15781831aec02c0ce9))


### Reverts

* **templates:** :recycle: switch back to daisyui carousel on mobile displays for home and fix color of indicator for active item ([17f492b](https://github.com/immanent-tech/foragd/commit/17f492bbfef98a9c149317907d75a0cf762eb5ff))

## [0.39.0](https://github.com/immanent-tech/foragd/compare/v0.38.0...v0.39.0) (2026-01-30)


### Features

* :sparkles: redo landing page screenshots ([a45765c](https://github.com/immanent-tech/foragd/commit/a45765ce30b154b3966eb8cc0f08ef3d9d45de48))
* **handlers:** :sparkles: automatically generate sitemap ([80b9b27](https://github.com/immanent-tech/foragd/commit/80b9b273370347bd181f74135a399b57344db61d))


### Bug Fixes

* **handlers:** :recycle: re-use notification handler in internal error handler ([3c777ae](https://github.com/immanent-tech/foragd/commit/3c777aed2a3a1e3afa83145ab5a0612168c2a44c))
* **templates:** :bug: correct CSS selector used for selecting all subscriptions/articles to mark on list pages ([e0ee248](https://github.com/immanent-tech/foragd/commit/e0ee248d8e658dcef1d215bd6ecd3b5d781d66ce))
* **templates:** :lipstick: make sure search suggestions is not bigger than visible screen ([3950290](https://github.com/immanent-tech/foragd/commit/39502908638f31d67696c8527182a8dfdef17345))

## [0.38.0](https://github.com/immanent-tech/foragd/compare/v0.37.0...v0.38.0) (2026-01-29)


### Features

* :sparkles: get a diverse sample of latest articles to display on home page ([c6406bb](https://github.com/immanent-tech/foragd/commit/c6406bbd9d7c9764768b94c1ea32785bd93eb26b))
* **handlers:** :sparkles: split rendering internal vs external pages ([22288b0](https://github.com/immanent-tech/foragd/commit/22288b0501fa75f5e2593807c4ec4a8871635e0d))


### Bug Fixes

* **templates:** :bug: fix handling of 3xx response with htmx ([59a1d26](https://github.com/immanent-tech/foragd/commit/59a1d26741eb7fd8da114233c6c6c160f126067b))
* **templates:** :bug: fix viewer footer positioning ([19b285f](https://github.com/immanent-tech/foragd/commit/19b285ff6d66724b35b94f19ea6f21b1474ce1a3))

## [0.37.0](https://github.com/immanent-tech/foragd/compare/v0.36.0...v0.37.0) (2026-01-28)


### Features

* :sparkles: add sitemap ([fe19ed1](https://github.com/immanent-tech/foragd/commit/fe19ed1baaf8da44f1348bcfca3541eea3837b97))

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
