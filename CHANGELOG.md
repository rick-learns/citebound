# Changelog

## 0.1.0 (2026-11-17)


### Features

* add make demo and make test, with the demo as a CI smoke test ([#43](https://github.com/rick-learns/citebound/issues/43)) ([b475f45](https://github.com/rick-learns/citebound/commit/b475f454888d5d63d02332e7c3369844ec829697))
* **answer:** answer with a language model, verified like any answer ([#37](https://github.com/rick-learns/citebound/issues/37)) ([9068d5d](https://github.com/rick-learns/citebound/commit/9068d5df066db9d3e8d792903cafdc64d2f4dcea))
* **answer:** answer with verified quotes and no API key ([#26](https://github.com/rick-learns/citebound/issues/26)) ([27bf090](https://github.com/rick-learns/citebound/commit/27bf090fc7776977dfc6895ae1bc81b5b09bddba))
* **answer:** report the model tokens each answer used ([#60](https://github.com/rick-learns/citebound/issues/60)) ([0f6d760](https://github.com/rick-learns/citebound/commit/0f6d76059ca0b6ac2f02e6305caaa55cccbcb293))
* **answer:** withhold the answer unless most claims verify ([#27](https://github.com/rick-learns/citebound/issues/27)) ([d8eb867](https://github.com/rick-learns/citebound/commit/d8eb867479f295814ccee438a1eb99f562c7b5b7))
* **cli:** check the quotes in any answer with citebound check ([#42](https://github.com/rick-learns/citebound/issues/42)) ([4381e9f](https://github.com/rick-learns/citebound/commit/4381e9f2608d1f906337c927613bcfad6b175967))
* **corpus:** pin 21 RFCs by checksum so quoted text can never drift ([#21](https://github.com/rick-learns/citebound/issues/21)) ([0884d2a](https://github.com/rick-learns/citebound/commit/0884d2ae1fef547e6f91ef424e326a0dee473d0d))
* **corpus:** split RFCs into citable paragraphs like RFC6749 §10.4 ¶2 ([#23](https://github.com/rick-learns/citebound/issues/23)) ([08d2294](https://github.com/rick-learns/citebound/commit/08d2294f37aaca0847692cacec1a31f7ac172fd8))
* **eval:** generate docs/scorecard.md and fail CI when it is stale ([#66](https://github.com/rick-learns/citebound/issues/66)) ([da3f052](https://github.com/rick-learns/citebound/commit/da3f052c3d09e601f9178b39f4557898fe92d982))
* **eval:** score an answerer's quotation validity and refusals ([#41](https://github.com/rick-learns/citebound/issues/41)) ([daa4a85](https://github.com/rick-learns/citebound/commit/daa4a854abda37e4896c0bd3254d5aec237a36fb))
* **eval:** score claim support from labeled claims ([#64](https://github.com/rick-learns/citebound/issues/64)) ([55cc514](https://github.com/rick-learns/citebound/commit/55cc51446ff98d48a25e7a395df14e69d4d85a88))
* **index:** find the right RFC paragraphs with BM25 over SQLite FTS5 ([#25](https://github.com/rick-learns/citebound/issues/25)) ([e9b8e8a](https://github.com/rick-learns/citebound/commit/e9b8e8a8971ed019902d41f9e404f7cc0c107908))
* **llm:** call Anthropic and OpenAI-compatible models over plain HTTP ([#29](https://github.com/rick-learns/citebound/issues/29)) ([7ca3ff7](https://github.com/rick-learns/citebound/commit/7ca3ff7c790cd92d39f25b98006414dd1597083e))
* **llm:** record model responses and replay them offline ([#35](https://github.com/rick-learns/citebound/issues/35)) ([4dbc2b3](https://github.com/rick-learns/citebound/commit/4dbc2b372e14130d0fa83b46bb082982f147a084))
* **server:** fall back to extractive answers when the daily token budget is used up ([#61](https://github.com/rick-learns/citebound/issues/61)) ([6188598](https://github.com/rick-learns/citebound/commit/61885984e3661023cf06625bdeaf1b44795c69ea))
* **server:** rate-limit each client and cap concurrent asks ([#59](https://github.com/rick-learns/citebound/issues/59)) ([3821a4a](https://github.com/rick-learns/citebound/commit/3821a4acd94b496455073d6b72d649bba687f603))
* **server:** serve corpus metadata and RFC sections over HTTP ([#28](https://github.com/rick-learns/citebound/issues/28)) ([2ae27b7](https://github.com/rick-learns/citebound/commit/2ae27b7dd2c51e5ef4ebe0e29f136cca0257b37b))
* **server:** serve the embedded web UI from citebound serve ([#53](https://github.com/rick-learns/citebound/issues/53)) ([097a677](https://github.com/rick-learns/citebound/commit/097a677d2369c2436b5dee2c0358243cf62198cb))
* **server:** stream verified answers as server-sent events ([#30](https://github.com/rick-learns/citebound/issues/30)) ([efc572d](https://github.com/rick-learns/citebound/commit/efc572d2b13b5dc6c29fc1bc95010080fc613bea))
* ship a distroless container image, smoke-tested in CI ([#67](https://github.com/rick-learns/citebound/issues/67)) ([1aff45d](https://github.com/rick-learns/citebound/commit/1aff45d0b98b8eef0ddd4a40aad3ff4b698266b2))
* **verifier:** strike every claim whose quote is not in the cited paragraph ([#24](https://github.com/rick-learns/citebound/issues/24)) ([47cdb13](https://github.com/rick-learns/citebound/commit/47cdb13b8640591ae77a7b684e0ee090122e0d28))
* **verify:** read an escaped double quote in a quote as a double quote ([#39](https://github.com/rick-learns/citebound/issues/39)) ([6154225](https://github.com/rick-learns/citebound/commit/615422579f2d241ae2556e6516fbe3b2035cbe87))
* **web:** add a typed client for the HTTP API ([#45](https://github.com/rick-learns/citebound/issues/45)) ([339247f](https://github.com/rick-learns/citebound/commit/339247f63172471200cc870e9b54c2b7c5059afe))
* **web:** add an SSE parser, a quote highlighter, and an element builder ([#44](https://github.com/rick-learns/citebound/issues/44)) ([4c7f142](https://github.com/rick-learns/citebound/commit/4c7f1427609f8956fe5fbd24afe40f3db0ef66a2))
* **web:** add the answer state reducer and the ask form ([#48](https://github.com/rick-learns/citebound/issues/48)) ([6fab6e8](https://github.com/rick-learns/citebound/commit/6fab6e85c9ecc532452e028b7bf6eddffdf41c69))
* **web:** add the answer view, masthead, and footer ([#50](https://github.com/rick-learns/citebound/issues/50)) ([e15fdbc](https://github.com/rick-learns/citebound/commit/e15fdbc73075832c1d94a5a4a38c213494bd3060))
* **web:** add the source panel ([#51](https://github.com/rick-learns/citebound/issues/51)) ([5a3333b](https://github.com/rick-learns/citebound/commit/5a3333b65a475e22d32993309cb9738bd9ab87e2))
* **web:** format answers and render claims ([#49](https://github.com/rick-learns/citebound/issues/49)) ([4ead24e](https://github.com/rick-learns/citebound/commit/4ead24e3cdd97c2708a9b56e1c4a564fe5a98b3b))
* **web:** style claims, notices, passages, and the source panel ([#55](https://github.com/rick-learns/citebound/issues/55)) ([bbacb7d](https://github.com/rick-learns/citebound/commit/bbacb7dcd79c6760d575aafaf04fb18c5cae90d4))
* **web:** style the page, masthead, and ask form ([#54](https://github.com/rick-learns/citebound/issues/54)) ([5e0aed4](https://github.com/rick-learns/citebound/commit/5e0aed4ca239c489871e3d9117d4e4b7fb89f73b))
* **web:** wire the page together and mount it ([#52](https://github.com/rick-learns/citebound/issues/52)) ([cadf705](https://github.com/rick-learns/citebound/commit/cadf7054f3d43c4b894c8cd20499340edb2e6180))


### Bug fixes

* **server:** report a replay miss as not_recorded, not internal ([#58](https://github.com/rick-learns/citebound/issues/58)) ([ee0585a](https://github.com/rick-learns/citebound/commit/ee0585a64c07e505b51fe57b700eb899c0bf59fd))


### Documentation

* add a failure gallery, each case pinned by a test ([#47](https://github.com/rick-learns/citebound/issues/47)) ([9793e2e](https://github.com/rick-learns/citebound/commit/9793e2eb942670e68aaafde62493037e9df6f4d8))
* add an architecture page with a pipeline diagram ([#68](https://github.com/rick-learns/citebound/issues/68)) ([df810a9](https://github.com/rick-learns/citebound/commit/df810a98fcc2e2e6aa555e398fc9ee679e8da366))
* explain how a change gets from an issue to a release ([#19](https://github.com/rick-learns/citebound/issues/19)) ([b850512](https://github.com/rick-learns/citebound/commit/b85051217312141f8178483d5f3c2b1ea6cb82d9))
* promise only what the verifier checks ([#31](https://github.com/rick-learns/citebound/issues/31)) ([7d9db86](https://github.com/rick-learns/citebound/commit/7d9db864357727b0e656959fa8d5b81a7bf0b072))
* show the web UI in the README ([#57](https://github.com/rick-learns/citebound/issues/57)) ([2ab1061](https://github.com/rick-learns/citebound/commit/2ab10610d427f96de37129bcc5db588b55a682d3))
