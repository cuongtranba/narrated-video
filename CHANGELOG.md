# Changelog

## [0.8.0](https://github.com/cuongtranba/narrated-video/compare/v0.7.0...v0.8.0) (2026-08-19)


### Features

* **kit:** the scaffold ships its lockfile ([#36](https://github.com/cuongtranba/narrated-video/issues/36)) ([3110f1b](https://github.com/cuongtranba/narrated-video/commit/3110f1b63f70bd6b19889db40ef8e90f44640fd2))


### Bug Fixes

* **ci:** the binaries gate no longer dies on fork and Dependabot PRs ([d2e63bb](https://github.com/cuongtranba/narrated-video/commit/d2e63bb1eef6ca7faec40543371d19647e4395a5))
* **kit:** CHK-38 — the Remotion family is pinned to one exact version ([#31](https://github.com/cuongtranba/narrated-video/issues/31)) ([2e8e27d](https://github.com/cuongtranba/narrated-video/commit/2e8e27d059e044ce231791a039e614927a646a51))
* **scenekind:** recognise a scene by its wrapper import, not only its packages ([#33](https://github.com/cuongtranba/narrated-video/issues/33)) ([991e6b2](https://github.com/cuongtranba/narrated-video/commit/991e6b23cea4fd2d96a8e8b76122b12bfc5405a3))
* **sync:** nv sync owns the GL renderer in remotion.config.ts too ([#35](https://github.com/cuongtranba/narrated-video/issues/35)) ([2b51bb8](https://github.com/cuongtranba/narrated-video/commit/2b51bb8050772cb836ae76204e680f23bd510ffa))

## [0.7.0](https://github.com/cuongtranba/narrated-video/compare/v0.6.0...v0.7.0) (2026-08-18)


### Features

* **checks:** CHK-28 — scene modules carry no wall-clock or nondeterministic motion ([#24](https://github.com/cuongtranba/narrated-video/issues/24)) ([7bc3da3](https://github.com/cuongtranba/narrated-video/commit/7bc3da3a132e2e92583620518d95c1718376be80))
* **checks:** CHK-31 & CHK-35 — assets must hold the frame; CI proves determinism ([#29](https://github.com/cuongtranba/narrated-video/issues/29)) ([359af44](https://github.com/cuongtranba/narrated-video/commit/359af4496e7156bdd85bc70970ee0f5e927e695d))
* **checks:** CHK-33 — space scenes require GL renderer; nv sync manages flag ([#30](https://github.com/cuongtranba/narrated-video/issues/30)) ([44ba41b](https://github.com/cuongtranba/narrated-video/commit/44ba41b89f046fd821867519717462f9bfb2222e))
* **diagram:** animated walk — Stagger nodes, Trace edges, derived order ([#27](https://github.com/cuongtranba/narrated-video/issues/27)) ([8b429a7](https://github.com/cuongtranba/narrated-video/commit/8b429a7ed558efa36f0470da864bdbd0146da334))

## [0.6.0](https://github.com/cuongtranba/narrated-video/compare/v0.5.0...v0.6.0) (2026-08-18)


### Features

* **diagram:** diagram data model — topology in config, labels in content, diagrams.ts derived by nv sync ([#22](https://github.com/cuongtranba/narrated-video/issues/22)) ([6f5bd22](https://github.com/cuongtranba/narrated-video/commit/6f5bd22e691ee8d4a9dd778ea40e80487b2b1053))
* **epic:** kit flow scene, SKILL routing, README diagram derivation ([#25](https://github.com/cuongtranba/narrated-video/issues/25)) ([3840193](https://github.com/cuongtranba/narrated-video/commit/38401939ee2c758f067acfc2c5b484964aca8cff))

## [0.5.0](https://github.com/cuongtranba/narrated-video/compare/v0.4.0...v0.5.0) (2026-08-18)


### Features

* **diagram:** static deterministic React Flow wrapper and diagram checks ([#21](https://github.com/cuongtranba/narrated-video/issues/21)) ([567cfc8](https://github.com/cuongtranba/narrated-video/commit/567cfc86f27bc3d85efd8f52bb1be9179176f51b))
* **motion:** motion vocabulary — Stagger, Trace, Focus, Emphasis, Count ([#20](https://github.com/cuongtranba/narrated-video/issues/20)) ([44bf12d](https://github.com/cuongtranba/narrated-video/commit/44bf12dea1eed6505d3ca3a51cbfffabcb7c95ae))
* **scenekind:** nv init --scene &lt;Id&gt; --kind &lt;text|flow|space&gt;, CHK-34 ([#18](https://github.com/cuongtranba/narrated-video/issues/18)) ([6f396aa](https://github.com/cuongtranba/narrated-video/commit/6f396aa45c5006a41636e1ffa88904a40f9143f5)), closes [#9](https://github.com/cuongtranba/narrated-video/issues/9)
* **space:** Space component and CHK-37 — Three.js scenes through the wrapper ([#23](https://github.com/cuongtranba/narrated-video/issues/23)) ([fc9095e](https://github.com/cuongtranba/narrated-video/commit/fc9095e8d3990b17c88746c9f5c1a8468c1c2f62))

## [0.4.0](https://github.com/cuongtranba/narrated-video/compare/v0.3.0...v0.4.0) (2026-08-17)


### Features

* **nv:** derive the pipeline, the script and the render target ([#4](https://github.com/cuongtranba/narrated-video/issues/4)) ([d57c6bf](https://github.com/cuongtranba/narrated-video/commit/d57c6bfe7ae19d3a7799e46eb0808893c88bfffe))

## [0.3.0](https://github.com/cuongtranba/narrated-video/compare/v0.2.0...v0.3.0) (2026-08-16)


### Features

* **kit:** default the template to Be Vietnam Pro with the vietnamese subset ([328a777](https://github.com/cuongtranba/narrated-video/commit/328a77729a719221ce646c7e7b67cda424686590))


### Bug Fixes

* **ci:** tell gh which repository to upload release assets to ([5771194](https://github.com/cuongtranba/narrated-video/commit/577119458d24300417146f3cd9626c643558ec55))

## [0.2.0](https://github.com/cuongtranba/narrated-video/compare/v0.1.0...v0.2.0) (2026-08-16)


### Features

* config-driven narrated video kit with a deterministic gate ([f25fe67](https://github.com/cuongtranba/narrated-video/commit/f25fe67d5f300cc2ea3ed17f4d51331b6382c70f))


### Bug Fixes

* accept the space form of a value flag ([3aa6327](https://github.com/cuongtranba/narrated-video/commit/3aa6327260e44e436809165c8bca0ad720e47779))
* ship the template's generated files as the tool derives them ([5c3d841](https://github.com/cuongtranba/narrated-video/commit/5c3d8415e76c6a82763ef7be695c05cba5c5cb67))
* stop documenting commands that do not exist ([b248495](https://github.com/cuongtranba/narrated-video/commit/b248495696a90d72431bdc5befa55c9324026527))
* strip the build id so binaries reproduce across machines ([2515e95](https://github.com/cuongtranba/narrated-video/commit/2515e953de9538074715696cdd970a457a44929f))
