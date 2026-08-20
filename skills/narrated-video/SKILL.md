---
name: narrated-video
description: Build and maintain narrated explainer videos in Remotion. Use for any request to plan, make, script, voice, localize or repair a video — explainer, demo, walkthrough, onboarding, launch, product tour, tutorial, release announcement; "make a video explaining X", "turn this README/feature/ADR into a video", "record a walkthrough of Y", "we need a 90-second explainer". Covers storyboarding the cut into beats, scaffolding the Remotion project, writing scenes and their motion, narration, voiceover and text-to-speech (ElevenLabs and offline providers), diagrams and 3D, subtitles and on-screen copy, adding a language or translating an existing video, adding or reordering a scene, committing audio, and diagnosing a video that already exists — timing that no longer matches the narration, a font that silently drops diacritics, missing or stale audio, cues that land at the wrong moment, a translation that overflows the frame, a cut that reads as a slideshow. Trigger on video work even when no tool, library or file name is mentioned.
---

# narrated-video

`nv` is a prebuilt binary that scaffolds a Remotion project, derives every scene's
length from measured audio, synthesizes narration, and gates the result with 42
checks. The project it writes contains no build tooling of its own — a broken
`node_modules` can change what renders, but it cannot change what the gate says.

The gate is strict about everything it can read and silent about two things it
cannot: **what the video shows**, and **what it looks like**. A project can pass
all 42 checks and still be a stack of headings read aloud. Designing the cut and
reading a frame are yours; both have a section below.

## The loop

```bash
nv status        # where the project is, and the one command to run next
<that command>
nv status        # again
```

Run it at the start of a turn and after every step. It derives each stage from
the files on disk — the config, the content, the manifests, the 42 checks — so it
is never a plan someone is keeping up to date; it is what the project is.

Do not reconstruct the running order from memory or from this file. Prose
describes the pipeline; `nv status` computes it, and only one of those two can be
wrong. `--json` carries the same data with `.next.command` spelled out.

## Routing

| Task | Read | Start with |
| --- | --- | --- |
| Anything, at any point | — | `nv status` |
| Turn a doc, feature or idea into a video | this file § Design the cut | decide the beats before touching a file |
| New video from nothing | this file, then `references/config-schema.md` | `nv init <dir>` |
| Add a scene | `references/scene-registry.md` | `nv init --scene <Id> [--kind text\|flow\|space]` |
| Make a scene carry its idea | `references/motion-vocabulary.md` | the beat/verb table below |
| Add a language / translate | `references/localization.md`, `references/fonts.md` | `locales.list` + `content/<code>.yaml` |
| Edit copy or narration | `references/localization.md` | `content/<locale>.yaml`, then `nv sync` |
| Publish the script for a human | `references/localization.md` | `nv script <locale> > docs/…` |
| The cut is the wrong length | `references/config-schema.md` | `video.targetDuration`, then `nv status` |
| See what it actually looks like | this file § Look at the frame | `bun run still --frame=<n>` |
| A check is failing | `references/validate-checks.md` | the remedy line the failure printed |
| Keep scenes looking like one video | `references/ui-consistency.md` | one accent, two steps of tone, colour never alone |
| Contrast / WCAG failed the gate | `references/ui-consistency.md` | raise the lightness gap in OKLCH |
| Draw an architecture / pipeline diagram | `references/diagrams.md` | `nv init --scene <Id> --kind flow`, use `<Diagram>` |
| Add a 3D scene | `references/3d.md` | `nv init --scene <Id> --kind space`, never use `useFrame` |
| Adapt Three.js advice found elsewhere | `references/threejs-bridge.md` | check it against the determinism rules before copying |

Also here: `references/timing-model.md` (why lengths are derived and how),
`references/tts-providers.md` (providers, models, the model that is wrong in a
way you cannot hear from a 200), `references/ui-consistency.md` (the design
floor and the contrast gate), `references/threejs-bridge.md` (which general
Three.js advice survives a headless deterministic renderer),
`references/troubleshooting.md`.

## Getting `nv`

```bash
npx skills add cuongtranba/narrated-video
export PATH="$HOME/.agents/skills/narrated-video/bin:$PATH"
```

The binaries are committed and arrive executable; nothing is built or downloaded
at install time. `bin/nv` is a POSIX shim that picks `darwin-arm64`,
`darwin-amd64`, `linux-amd64` or `linux-arm64`. Windows is not supported yet, and
the shim says so rather than failing obscurely.

Commands: `init [dir]` (`--scene <Id>`, `--kind text|flow|space`), `status`
(`--json`), `sync`, `validate` (`--json`), `voiceover [locale…]` (`--force`),
`script [locale]`, `version`. Run any of them from anywhere inside a project —
the root is found by walking up, like `git`.

Flags are parsed by hand and an unrecognised one is ignored silently, so a
misspelled `--jsonn` gets you human output and exit 0 rather than an error. Only
`--scene` and `--kind` take a value; `--json` and `--force` are bare.

## Design the cut before you write it

A video is a sequence of beats, one idea each. Decide the beats first — on paper,
in your head, in the plan you show the user — because the alternative is
discovering at scene six that beats two and four were the same point. Scene count
follows from the beats; it is not a number to pick up front.

For each beat, name three things before you write the module:

1. **The one sentence it exists to land.** That sentence becomes its narration
   line. If you cannot write it, the beat is not a scene yet.
2. **Its kind** — `text`, `flow` or `space`. This decides which template the scene
   starts from and which packages get installed.
3. **The verb that carries it** — what moves, and when. A scene whose only motion
   is a heading and one fade is a slide; five of those in a row is a slideshow with
   a voice on top, which is the failure mode this kit exists to avoid and the one
   the gate cannot see.

### Choosing the kind and the verb

| When the beat is… | Kind | Reach for |
| --- | --- | --- |
| a claim, a definition, the sentence a term turns on | `text` | `Emphasis` on that term — `underline`, `tint` or `weight` |
| a list, a sequence of steps, a set of options | `text` | `Stagger`, so items land as they are spoken rather than all at once |
| a number that is the point | `text` | `Count` climbing to it; `Mono` for anything the system itself printed |
| how parts connect — a pipeline, a state machine, an architecture | `flow` | `<Diagram reveal={{at, through}}>` walking the graph in narration order |
| **something travelling between the parts** — requests, events, writes, retries | `flow` | `<Diagram flow>` — packets run the edges once each is drawn |
| one node in a graph while the voice is on it | `flow` | `<Diagram subject="<nodeId>">` — every other node dims to 0.35 |
| an edge that connects but carries nothing | `flow` | that edge's `flowing: false`, so the moving ones mean something |
| a topology that is spatial — regions, replicas, tiers, a mesh | `space` | `<Space turntable>`, which rotates the subject so depth reads |
| **data crossing a distance in space** | `space` | `<Beam from to>` — a link with packets running it |
| machines, datastores, a cluster you can point at | `space` | `<Rack>` and `<Datastore>` — never a bare sphere or cube standing in for a concept |
| a shape, a volume, a structure to look around | `space` | `<Space>`, every transform a function of `useCurrentFrame()` |
| a detail inside something already on screen | any | `Focus` pushing in on a pivot; a second `Focus` at `scale: 1` pulls back out |
| a status, a verdict, a pass/fail | `text` | `Pill` — label plus dot, so the meaning survives greyscale |
| an aside, a caveat, a source | any | `Stage`'s `footnote`, pinned to the bottom safe edge |
| something that must leave before the next thing lands | any | `Reveal until={…}` — the only exit in the kit |

A scene may draw on more than one kind — a diagram inside a 3D stage — and it gets
the packages of all of them.

### Reach for a diagram or a stage before reaching for a paragraph

The two heaviest kinds are the two most often skipped, because a paragraph is
always *available* and a graph has to be laid out. Prefer them anyway whenever
the beat is about **structure or movement**, because a sentence describing four
hops asks the viewer to hold four things they cannot see, and a diagram just
shows them.

Concretely, treat these as defaults rather than options:

- Any beat naming three or more components and how they relate → a `flow` scene,
  not a bulleted list. A list of components loses the arrows, and the arrows were
  the content.
- Any beat where something **moves** — a request, a message, a write, a retry, a
  deploy — → `<Diagram flow>` or `<Beam>`. A static arrow says two things are
  connected; a moving packet says which way traffic goes, that it is continuous,
  and where it stops. `flowing: false` on the edges that carry nothing is what
  makes the moving ones legible.
- Any beat about **arrangement in space** — regions, zones, replicas, layers,
  anything where "next to" and "far from" carry meaning → `space` with
  `turntable`. Depth is the one thing a flat diagram cannot show, and a slow
  rotation is what makes a viewer read a cluster as a volume rather than a
  scatter of boxes.

The cost is real — a diagram needs positions, a 3D scene needs a camera — so
spend it where structure is the point and stay in `text` where a sentence is
genuinely the whole idea.

### Everything you can reach for

Knowing the inventory is most of the job. Every item below is built, typed and
tested, and most of them appear in **no template**, so copying a template gets you
none of them.

- `../components/stage` — `Stage` (`heading`, `headingAt`, `footnote`,
  `footnoteAt`, `contentInsetBottom`). The frame every scene lives in: background,
  locale-correct font, safe area.
- `../components/primitives` — `Reveal` (the one entrance; `rise`, `until`),
  `Rule` (a divider that draws itself in), `Card`, `Mono`, `Pill`, and the tokens
  `SAFE`, `SIZE`, `RADIUS`, `EASE_OUT`, `HAIRLINE`.
- `../components/motion` — `Stagger`, `Trace`, `Flow`, `Focus`, `Emphasis`,
  `Count`. `Trace` draws a path once; `Flow` runs packets along one forever.
  Both take any SVG `d`, not only diagram edges: underlines, brackets,
  connectors, a route across a map.
- `../components/diagram` — `Diagram` (`reveal`, `flow`, `subject`),
  `DiagramGraph` (whose `order` decouples the walk from declaration order), and
  per-edge `flowing`.
- `../components/space` — `Space` (`height`, `camera`, `turntable`), the object
  vocabulary `Datastore` and `Rack`, and `Beam`, a 3D link with packets
  travelling it. `Space` owns the canvas, background and lighting. A bare
  primitive standing in for a concept is the 3D equivalent of a slideshow.
- `../color` — `THEME_3D`, the palette converted for three.js, and
  `toThreeColor`. **Materials and lights take `THEME_3D`; everything else takes
  `THEME`.** Three.js cannot parse the OKLCH the config is written in — it warns
  and silently renders white, so a 3D scene built from `THEME` is the wrong
  colour at exit 0.
- `../timing` — `at(fraction, durationInFrames)` for a single cue, and
  `beatSpans(weights, durationInFrames)` when one scene holds several beats.
  Weights are relative, so `[1, 1, 3]` gives the third beat triple the room in
  every language.
- `../generated/theme` — `THEME.background`, `.foreground`, `.muted`, `.surface`,
  `.border`, `.accent`. Colour comes from the config; the type scale and safe area
  are the components' own, which is why they are tokens and not settings.

Props and exact behaviour: `references/motion-vocabulary.md` for the five motion
primitives, `references/scene-registry.md` for the scene contract and `Stage`,
then `references/diagrams.md` and `references/3d.md`.

### Cueing

Three tiers, and every cue is one of them:

- `at={0}` — settled before the first word.
- `at={leadFrames}` — lands exactly as the voice starts.
- `at={at(CUE.x, durationInFrames)}` — everything else, as a fraction.

Collect the fractions in one `CUE` object at the top of the module so the scene
reads as a score rather than as forty copies of `interpolate`. Fractions are what
let a translation re-time its own cues instead of drifting away from the sentence
explaining them; CHK-22 rejects a numeric literal in that second argument.

## New video

```bash
nv init my-video && cd my-video
# wrote 30 files to my-video

nv status        # ▸ voiceover — 0/2 scenes measured; next: nv voiceover
nv voiceover     # provider `silence`: no API key, no network
# Title  2.22s  67f
# Outro  1.49s  45f
# then it regenerates src/generated/ automatically

nv status        # ▸ render — gate green; next: bun install && bun run render
nv validate      # exit 0 — "42 checks passed"
```

**Do not assemble the scaffold yourself.** Do not run `create-video`. Do not
`remotion add` anything. Do not hand-write `remotion.config.ts` or `tsconfig.json`.
Do not create a `scripts/` directory, install `ajv`, `@remotion/media-parser` or a
YAML library. `nv init` already wrote all of it, pinned and consistent — assembling
it by hand is what cost the project this tool came from roughly 29 tool calls of
pure plumbing, most of it spent undoing a template's defaults.

What `nv init` gives you: `video.config.yaml`, `video.schema.json`,
`content/en.yaml`, two example scenes (`Title`, `Outro`) plus the three scene-kind
templates (`_template.tsx`, `_template.flow.tsx`, `_template.space.tsx`),
the components, the generated files, `package.json` with `studio` / `render` /
`still` / `typecheck` / `lint`, eslint, tsconfig, `.gitignore`, `.env.example`,
and a project `README.md`.

Then: edit `video.config.yaml` (id, size, fps, theme, scenes, locales) and
`content/en.yaml` (the spoken lines and every on-screen string), write the
scenes, `nv sync`, `nv validate`.

Two orderings that each save a round trip:

- **YAML before the scene.** The `Copy` type is derived from the default locale's
  file, so a `useCopy()` field that is not yet in `content/en.yaml` is a type error
  until the next `nv sync`.
- **`nv sync` after `nv init --scene`.** That is the one command which does not
  sync on its own (`nv init` and `nv voiceover` both do). A `space` scene needs the
  sync before anything renders: the GL backend CHK-33 requires is written into
  `remotion.config.ts` and the render scripts by sync, and without it 3D is a black
  rectangle at exit 0.

## The script

The spoken lines live in `content/<locale>.yaml` and nowhere else. To hand a
human something readable, generate it:

```bash
nv script vi > docs/video-scripts/explainer-vi.md
```

Do not write the script as prose in a doc and copy it into the yaml afterwards.
That is two files which agree until the first edit, and the edit always comes —
a line written to be read is not yet a line written to be spoken. Regenerate
instead; there is nothing to keep in sync.

Declare the length you were asked for and let the tool hold you to it:

```yaml
video:
  targetDuration:
    minSeconds: 120
    maxSeconds: 180
```

`nv status` shows the projected length against that window from the first draft
on, and CHK-26 fails the gate once every scene is measured. It stays silent
while the length is only an estimate — an estimate carries real error, and
failing on a number that is not yet true would block work on a guess.

The measured speaking rate is about 16 characters a second, so a 150-second cut is
roughly 2,400 characters of narration spread across the beats. Budget against that
while the beats are still a list; trimming a finished script costs more.

## The timing model

`duration = leadFrames + narration + tailFrames`, with the cross-fade repaid once
per scene boundary. Only the two pads are anyone's decision, and they are set once
in `defaults` (14 / 24) with a per-scene override where a scene should outlast its
line. Narration length is **measured from the mp3 that exists**, never from what a
provider claims it produced.

Reveals inside a scene are cued as **fractions** of the scene —
`at(0.45, durationInFrames)` — so a translation that runs 12% longer re-times its
own cues instead of drifting away from the sentence explaining them.

`src/generated/timeline.ts` holds the frame table. It is written by `nv sync` and
**never edited by hand**; CHK-01 regenerates it in memory and compares bytes, so a
hand-edit or a stale file is a non-zero exit rather than a scene that renders
perfectly at every frame while sitting 14 frames off its audio. Details and the
measured evidence: `references/timing-model.md`.

## Narration

The API key is read from the environment variable **named** by `tts.apiKeyEnv`.
It never goes in the config, never in a content file, never inline in a command —
CHK-03 scans every tracked file for key shapes because the project this came from
leaked its key through a dozen shell commands, and a key in a commit is only fixed
by rotating it.

The `model` is chosen per locale and choosing wrong fails **silently**: a
multilingual model can speak a tonal language with the tones wrong and still
return HTTP 200 with a plausible duration. Nothing but a native ear catches it, so
`nv` keeps a per-locale allow-list and refuses anything not on it — including
models nobody has judged yet. Read `references/tts-providers.md` before picking a
model for a new language.

Three providers are registered. Default is `silence`: correctly-sized silent mp3s,
byte-identical on every machine, no account and no network. Everything except the
voice works from a fresh clone, and the render wears an `UNVOICED` badge until real
audio exists. `say` is macOS's built-in voice — a real read with no API key, useful
for hearing pacing, but non-deterministic, so CHK-19 refuses to let it be
committed. `elevenlabs` is the shippable one.

`nv voiceover <locale>` re-synthesizes just that language and leaves every other
locale's measurements alone. `--force` waives only the pre-flight cost estimate
against `tts.costCapUsd`; it does not force re-synthesis.

## Look at the frame

`nv validate` reads no pixels — deliberately, and that is what lets it run in a
bare container before `bun install`. The cost is that nothing in it can tell you
the heading overlaps the diagram, the Vietnamese line wrapped to three, the
`Stagger` finished long before the sentence did, or that the 3D scene came out
black. Those are the failures a viewer notices first, and a frame is the only thing
that reports them. Render one and read it:

```bash
bun install
bun run still --frame=200        # → out/explainer.png
```

Use `bun run still` rather than a bare `remotion still`: `nv sync` keeps the
composition id and any `--gl=angle` inside that script, so it stays aimed at the
right target as the config changes. Checking one scene in isolation means
bypassing it —

```bash
bunx remotion still <SceneId> out/<SceneId>.png --frame=40
```

— because every scene is registered a second time as its own composition under its
bare id, timed with the real `leadFrames` it gets in the cut. Copy across whatever
flags the `still` script currently carries; a project with a space scene needs
`--gl=angle` or you will photograph a black rectangle at exit 0.

Frames worth spending a render on: `leadFrames` (the first word), each fraction in
`CUE`, and the last frame before a boundary. For a translation, the same frames in
that locale's composition — non-default locales are `<video.id>-<code>`.

`bun run studio` when you want to scrub rather than sample.

### A still can disagree with the render

For anything whose layout is **measured** rather than declared — React Flow
handles above all — a still is not evidence about the video. `remotion still`
renders one settled frame and measures correctly; `remotion render` walks from
frame zero, may measure mid-entrance, and caches the result for the rest of the
scene. A diagram's edges hung 24px below their nodes for several rounds of
"fixed" because every check was run against stills.

When a layout bug survives a fix, extract the frame from the mp4 and measure
that:

```bash
FF=node_modules/@remotion/compositor-darwin-arm64
DYLD_LIBRARY_PATH=$FF $FF/ffmpeg -ss <seconds> -i out/explainer.mp4 -frames:v 1 out/frame.png
```

## The gate

`nv validate` runs all 42 checks — no fail-fast — and exits 1 if any fail. Each
failure prints where it is and one imperative remedy line. `--json` for CI.

It reads no pixels, asks no model to judge prose, and touches no network or API
key. That is the only property the exit code has, and it is what lets the gate run
in a bare container on a fresh clone before `bun install`.

When a check fails, fix what it found. Do not fix a check by weakening it —
loosening `expansionFactor` to silence CHK-14 or dropping a term from `glossary`
to silence CHK-16 buys a green exit and ships the bug. Every check exists because
something shipped wrong; `references/validate-checks.md` says what, per check.

Four whose cause is not obvious from the finding, all worth knowing before you
write rather than after:

- A scene may import `generated/theme` and `generated/content`, never
  `generated/timeline`, `generated/registry` or the config (CHK-21). Length arrives
  as a prop; a number inside a module is a number nothing checks.
- No `Math.random`, `Date.now` or `setTimeout` in a scene (CHK-28). Frames are
  captured in parallel browser tabs, so anything not derived from
  `useCurrentFrame()` makes two renders of the same frame disagree. Seeded
  randomness is `random()` from `remotion`.
- React Flow only through `<Diagram>` (CHK-36), Three.js only through `<Space>`
  (CHK-37). The wrappers are where the determinism guards live.
- Every `remotion` and `@remotion/*` package on one exact version (CHK-38). Two
  copies of Remotion break React context while the render still exits 0.
- The palette is scored for contrast (CHK-39, CHK-40). A video cannot be zoomed
  or restyled by the person watching it, so 4.5:1 on text and 3:1 on the accent
  are enforced rather than advised. `references/ui-consistency.md` says how to
  raise a failing pair — and why deleting the key is not how.

Four failures no check covers, all of which render and exit 0:

- `<Diagram>` with neither `reveal` nor `flow` draws no edges at all.
- `<Diagram>` fills its parent, so it needs an ancestor with an explicit height.
  `<Space>` is the exception — it takes its own `height` and must **not** be
  wrapped in a sized div, because a WebGL canvas cannot flex and the mismatch
  drifts the scene off-centre.
- A three.js material or light given a `THEME` colour renders **white**. Use
  `THEME_3D`.
- `nv sync` emits diagram nodes sorted by id, so a config-declared graph needs an
  explicit `order` or the walk visits them alphabetically.

## Done means

1. `nv validate` exits 0 — "42 checks passed".
2. No `UNVOICED` badge in the render. The badge cannot be switched off by a flag,
   only by having measured audio for every narrated scene, because a silent draft
   that looks finished is how a wrong cut escapes.
3. You have looked at frames — at least the first word and one cue per scene — and
   what you saw is what the narration says.
4. `bun run render` produces the file (`out/explainer.mp4` by default).
5. `nv status` reports nothing outstanding.

`bun run typecheck` and `bun run lint` cover what the gate deliberately does not:
the gate answers questions about the project's data, TypeScript answers questions
about its code.

Run both through `bun run`, never `bunx tsc`. The project installs TypeScript 7
for checking and TypeScript 6 for the lint chain — typescript-eslint cannot load
against the 7.0 API yet — and a bare `tsc` resolves to the 6.0 binary. The
scripts name the right compiler; `kit/README.md` explains why there are two.
