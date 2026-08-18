---
name: narrated-video
description: Build and maintain narrated explainer videos in Remotion. Use for any request to make, script, voice, localize or repair a video — explainer, demo, walkthrough, onboarding, launch, product tour, tutorial, release announcement; "make a video explaining X", "turn this README/feature/ADR into a video", "record a walkthrough of Y", "we need a 90-second explainer". Covers scaffolding the Remotion project, writing scenes, narration, voiceover and text-to-speech (ElevenLabs and offline providers), subtitles and on-screen copy, adding a language or translating an existing video, adding or reordering a scene, committing audio, and diagnosing a video that already exists — timing that no longer matches the narration, a font that silently drops diacritics, missing or stale audio, cues that land at the wrong moment, a translation that overflows the frame. Trigger on video work even when no tool, library or file name is mentioned.
---

# narrated-video

`nv` is a prebuilt binary that scaffolds a Remotion project, derives every scene's
length from measured audio, synthesizes narration, and gates the result with 28
checks. The project it writes contains no build tooling of its own — a broken
`node_modules` can change what renders, but it cannot change what the gate says.

## The loop

```bash
nv status        # where the project is, and the one command to run next
<that command>
nv status        # again
```

Run it at the start of a turn and after every step. It derives each stage from
the files on disk — the config, the content, the manifests, the 31 checks — so it
is never a plan someone is keeping up to date; it is what the project is.

Do not reconstruct the running order from memory or from this file. Prose
describes the pipeline; `nv status` computes it, and only one of those two can be
wrong. `--json` carries the same data with `.next.command` spelled out.

## Routing

| Task | Read | Start with |
| --- | --- | --- |
| Anything, at any point | — | `nv status` |
| New video from nothing | this file, then `references/config-schema.md` | `nv init <dir>` |
| Add a scene | `references/scene-registry.md` | `nv init --scene <Id> [--kind text\|flow\|space]` |
| Add a language / translate | `references/localization.md`, `references/fonts.md` | `locales.list` + `content/<code>.yaml` |
| Edit copy or narration | `references/localization.md` | `content/<locale>.yaml`, then `nv sync` |
| Publish the script for a human | `references/localization.md` | `nv script <locale> > docs/…` |
| The cut is the wrong length | `references/config-schema.md` | `video.targetDuration`, then `nv status` |
| A check is failing | `references/validate-checks.md` | the remedy line the failure printed |
| The scene is flat / reads as a slideshow | `references/motion-vocabulary.md` | `Stagger`, `Trace`, `Focus`, `Emphasis`, `Count` in `motion.tsx` |
| Draw an architecture / add a 3D scene | `references/3d.md` | `nv init --scene <Id> --kind space`, never use `useFrame` |

Also here: `references/timing-model.md` (why lengths are derived and how),
`references/tts-providers.md` (providers, models, the model that is wrong in a
way you cannot hear from a 200), `references/troubleshooting.md`.

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
nv validate      # exit 0 — "31 checks passed"
bun install && bun run studio
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

Default provider is `silence`: correctly-sized silent mp3s, byte-identical on
every machine, no account and no network. Everything except the voice works from
a fresh clone, and the render wears an `UNVOICED` badge until real audio exists.

## The gate

`nv validate` runs all 31 checks — no fail-fast — and exits 1 if any fail. Each
failure prints where it is and one imperative remedy line. `--json` for CI.

It reads no pixels, asks no model to judge prose, and touches no network or API
key. That is the only property the exit code has, and it is what lets the gate run
in a bare container on a fresh clone before `bun install`.

When a check fails, fix what it found. Do not fix a check by weakening it —
loosening `expansionFactor` to silence CHK-14 or dropping a term from `glossary`
to silence CHK-16 buys a green exit and ships the bug. Every check exists because
something shipped wrong; `references/validate-checks.md` says what, per check.

## Done means

1. `nv validate` exits 0 — "31 checks passed".
2. No `UNVOICED` badge in the render. The badge cannot be switched off by a flag,
   only by having measured audio for every narrated scene, because a silent draft
   that looks finished is how a wrong cut escapes.
3. `bun run render` produces the file (`out/explainer.mp4` by default).
4. `nv status` reports nothing outstanding.

`bun run typecheck` and `bun run lint` cover what the gate deliberately does not:
the gate answers questions about the project's data, TypeScript answers questions
about its code.
