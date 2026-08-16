# Timing model

Answers: how long does a scene last, who decides, and why is the frame table
generated code rather than a JSON file or a `calculateMetadata` callback.

## The formula

```
durationInFrames = leadFrames + narrationFrames + tailFrames
totalFrames      = Σ durationInFrames − transitionFrames × (sceneCount − 1)
```

- `leadFrames` — silence at the head, and the frame the `<Audio>` mounts at.
  Default 14.
- `narrationFrames` — `ceil(seconds × fps)`, where `seconds` is measured from the
  mp3's own frame headers by `nv voiceover`. Never a number a provider reported.
- `tailFrames` — silence after the voice stops, so the frame settles before the
  cut. Default 24.
- `transitionFrames` — the cross-fade, which overlaps two scenes, so it is
  subtracted once per boundary. Default 14.

A scene with `narrated: false` has no lead, no tail and no audio; it declares
`durationInFrames` directly. A narrated scene declaring `durationInFrames` fails
CHK-12 — that is precisely how a hand-maintained frame table grows back after the
derivation removed it.

## Why derived: the measured evidence

The project this tool came from kept a `SCENE_FRAMES` table by hand and rewrote it
**seven times** — once per text-to-speech run, because synthesis is not
deterministic and identical text re-synthesized shifted lengths by up to 14 frames.
Each rewrite was a human recomputing `narration + pad`.

Its final numbers, 8 scenes × 2 locales, against the audio it actually shipped
(these are the fixture `internal/timing/testdata/reference-{en,vi}-manifest.json`,
and `internal/timing/timing_test.go` asserts the derivation reproduces every one
of them exactly):

| Scene | en narration | en table | en pad | en tail | vi narration | vi table | vi pad | vi tail |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Title | 160 | 190 | 30 | 16 | 155 | 185 | 30 | 16 |
| Marathon | 374 | 410 | 36 | 22 | 395 | 432 | 37 | 23 |
| Arm | 392 | 437 | 45 | 31 | 394 | 432 | 38 | 24 |
| Iteration | 1145 | 1209 | 64 | **50** | 1283 | 1350 | 67 | **53** |
| BlockedTools | 234 | 270 | 36 | 22 | 275 | 310 | 35 | 21 |
| Decision | 575 | 615 | 40 | 26 | 623 | 662 | 39 | 25 |
| Termination | 491 | 530 | 39 | 25 | 500 | 542 | 42 | 28 |
| Outro | 144 | 174 | 30 | 16 | 146 | 178 | 32 | 18 |

`pad = table − narration`; `tail = pad − 14` once the lead is separated out.
Totals: en 3737, vi 3993.

Sixteen hand-maintained numbers are **one default plus one override**. The tails
cluster at 16–31 (median 23.5 — which is where the default 24 comes from) except
`Iteration`, which holds ~50 frames past its last word because that scene's
argument *is* the silence. In YAML:

```yaml
defaults:
  leadFrames: 14
  tailFrames: 24
scenes:
  - Title
  - id: Iteration
    tailFrames: 50    # holds on the context meter in silence — the drop is the point
  - Outro
```

The reason the original table was kept by hand — *a scene must be able to outlast
its narration* — is preserved, and now visible in the config instead of recoverable
only by opening `manifest.json` and subtracting.

## The lead exists for a reason

The cross-fade overlaps two scenes. A narration track mounted at the scene's frame
0 therefore starts speaking while the **previous** scene is still on screen,
mid-fade. The reference project had exactly this bug on every line; it survived
review only because synthesized speech tends to open on a beat of near-silence.

`leadFrames` both extends the scene and offsets the `<Audio>` mount
(`<Sequence from={leadFrames}>` in `src/Video.tsx`), which turns the accident into
a checkable guarantee: **no narration begins before its scene is fully opaque**,
enforced as `leadFrames >= transitionFrames` by CHK-13.

Inside a scene, `leadFrames` is also the cue for anything that should land exactly
as the voice does — a rule drawing in, an underline. Anything that must already be
on screen when the sentence begins is cued at `0`.

## Estimated vs measured vs literal

`SceneTiming.source` is one of three values, and the render reads it:

| source | means | badge |
| --- | --- | --- |
| `measured` | taken from an mp3 on disk | — |
| `estimated` | predicted from character count, no audio yet | `UNVOICED` |
| `literal` | unnarrated scene, length declared in the config | — |

Estimation is `runeCount / charsPerSecond × fps`, accurate to roughly ±15% —
enough to review layout, not enough to ship. Measured speaking rates from the
reference cut: **en 16.17 chars/s, vi 15.55 chars/s** (runes, not bytes; byte
length overstates a non-ASCII script by half again).

A missing manifest is a normal state, not an error. A fresh clone with no API key
must still open in Studio and render — it degrades to an estimate and says so.

Three states, and what each one looks like:

| State | timeline says | Studio / render | validate |
| --- | --- | --- | --- |
| audio present, hash matches | `measured`, `hasAudio: true` | plays | pass |
| manifest present, mp3s absent | `measured`, `hasAudio: false` | silent + badge | CHK-06 |
| no manifest at all | `estimated` | silent + badge | CHK-05 |

The `UNVOICED — 3/8 scenes estimated · nv voiceover` badge renders in
`remotion render` exactly as it does in Studio, deliberately, and there is no prop
or flag to suppress it. Run `nv voiceover` and it disappears on its own.

## Why generated code, and not the alternatives

`nv sync` writes `src/generated/timeline.ts` and it is committed. `Root.tsx`
imports it synchronously, so `durationInFrames` is a constant at composition
registration.

| Approach | Cost |
| --- | --- |
| **generated module** (chosen) | duration constant at registration; ordinary hot-reload; one weakness — staleness — closed by CHK-01 byte-comparing a fresh derivation |
| static JSON import | needs a literal specifier per locale, so adding a language means editing TypeScript; a missing manifest becomes a build error that stops Studio opening at all |
| `calculateMetadata` | an async resolve on every Studio boot and every `still`, × (1 + N) compositions, to re-read a file that changes twice a month; a throw there removes the composition from the list, which reads as "the video is gone"; Studio does not re-resolve on a `public/` change, so it is *worse* hot-reload than what it replaces |

`calculateMetadata` remains the right escape hatch for the one case this design
does not cover: a duration that genuinely depends on **render-time props** — a
per-render clip length passed in from a CLI `--props`, for instance. Nothing in
the kit needs it; reach for it only when the length is not knowable at sync time.

A generated table is still a table. The difference is that no human types into it
and its provenance is verified. The human was the derivation function; that was
the bug.

## Beat spans

`beatSpans(weights, durationInFrames)` in `src/timing.ts` divides a scene into
consecutive spans by **relative weight**: a beat that should hold twice as long as
its neighbour says `2`, and stays twice as long when the scene grows in another
language. Each span's edges are rounded from the *cumulative* fraction rather than
from its own width, so the spans tile the scene exactly — rounding each width
independently leaves one-frame gaps that flicker at the seams.

## What regenerates when

| You changed | Run |
| --- | --- |
| `video.config.yaml` | `nv sync` |
| `content/<locale>.yaml` (copy) | `nv sync` |
| `content/<locale>.yaml` (a narration line) | `nv voiceover <locale>` — it re-syncs itself |
| nothing, but CHK-01 fails | `nv sync` |

`nv voiceover` always regenerates afterwards. Leaving that to the operator is how
a project ends up with scenes timed to audio it no longer has.
