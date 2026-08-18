# The 28 checks

Answers: what each check reads, when it fails, and what to do about it.

`nv validate` runs every check — there is no fail-fast — and exits 1 if any fail.
`--json` emits the full `[{ID, Title, OK, Findings, Remedy}]` array. Findings are
sorted, and results come back in registry order, so the output reads the same way
twice.

Remedies below are the exact strings the tool prints.

## Contents

- [Generation and config: CHK-01 – CHK-04](#generation-and-config)
- [Audio: CHK-05 – CHK-10](#audio)
- [Copy and localization: CHK-11, CHK-14 – CHK-17](#copy-and-localization)
- [Scene shape and timing: CHK-12, CHK-13, CHK-25, CHK-26](#scene-shape-and-timing)
- [Fonts: CHK-18](#fonts)
- [Provider and spend: CHK-19, CHK-20](#provider-and-spend)
- [Source discipline: CHK-21 – CHK-24, CHK-27, CHK-34](#source-discipline)
- [What is deliberately excluded](#what-is-deliberately-excluded)

## Generation and config

### CHK-01 — generated files match the config and the measured audio

Reads every file under `src/generated/` and re-derives all four in memory from the
config, the content files and the manifests, then compares **bytes**.

Fails when any differs — a stale file, a hand-edit, or a config change not synced.

Remedy: `run: nv sync`

This is codegen's one weakness closed. A scene that drifted by fourteen frames
renders perfectly at every single frame; nothing but the comparison finds it. The
generation is pure — sorted keys throughout — so the byte comparison is meaningful
rather than flaky.

### CHK-02 — video.config.yaml satisfies the published schema

Reads `video.config.yaml` and validates it against `video.schema.json`, reporting
**every** violation at once rather than stopping at the first.

Remedy: `correct the fields named above — every one is documented in references/config-schema.md`
(or, if the file is missing: `create video.config.yaml, or run: nv init`)

### CHK-03 — no secret is written down anywhere

Reads `tts.apiKeyEnv` plus every git-tracked text file under 1 MiB, and the config
and content files whether tracked or not (a project not yet committed is exactly
when a key is most likely pasted in "just to try it").

Fails when `apiKeyEnv` is not an environment variable name, or when any scanned
file contains something shaped like a live credential: `sk_…`, `sk-…`, `xi-…`,
`AIza…`, `ghp_…`.

Remedy: `reference the secret by name instead — tts.apiKeyEnv: ELEVENLABS_API_KEY — and rotate anything already committed`

Widened past YAML on purpose: the reference project leaked its key through about a
dozen shell commands, not through the config.

### CHK-04 — every locale's TTS model is cleared for that language

Reads `tts.provider` and `tts.voices` and consults the provider's model policy.

Fails when a locale has no voice, when its model is **denied**, or when its model
is merely **unlisted**. Unlisted fails too: a model never judged by ear for a
language must not default into use just because the request would succeed.

Remedy: `pick a model cleared for the language, or clear this one by ear and record it in the provider's policy`
(unknown provider: `set tts.provider to a provider the tool ships — see references/tts-providers.md`)

## Audio

### CHK-05 — every narrated scene is timed from real audio

Reads each locale's derived timeline; fails on any scene whose `source` is
`estimated`.

Remedy: `run: nv voiceover -- <locale>`

An estimate from character count is accurate to ~±15% — enough to review layout,
not enough to ship. The frame and the voice drift apart across the scene.

### CHK-06 — every narrated scene has its audio file

Fails when a narrated scene's `public/voiceover/<locale>/<Id>.mp3` is absent or
empty.

Remedy: `run: nv voiceover -- <locale> (or fetch the committed audio)`

Separate from CHK-05 because the remedy differs: a clone that skipped large files
needs them fetched, not re-synthesized.

### CHK-07 — every audio file was spoken from the line that is there now

Compares `sha256(narration line)` against the manifest entry's `sha256`.

Remedy: `run: nv voiceover -- <locale>`

**The one drift a render cannot reveal.** Edit a line, forget to regenerate, and
the old take still plays: the voice contradicts the frame, and there is nothing on
screen to show it.

### CHK-08 — the audio on disk was made with a model cleared for its language

Reads the `provider` and `model` each manifest entry recorded, and judges those —
not the config.

Remedy: `re-run: nv voiceover -- <locale>, without a model override`

Checking only the config proves what was *intended*. An environment override can
synthesize a locale with a model that mispronounces it, and the file is
indistinguishable from a good one without listening. An entry recording no
provider or model fails too: what produced it cannot be judged.

### CHK-09 — measurements were taken at the video's frame rate

Compares each manifest's `fps` against `video.fps`.

Remedy: `re-measure at the current frame rate: nv voiceover -- <locale>`

Change fps in the config and every committed frame count silently means something
else.

### CHK-10 — narration lines and narrated scenes correspond

Fails on a narration key with no narrated scene behind it, and on a narrated scene
with no line, in any locale.

Remedy: `add the missing line, or mark the scene unnarrated with durationInFrames in video.config.yaml`

A narration key with no scene is a line nobody hears; a narrated scene with no line
is silence where the argument was. Both are quiet.

## Copy and localization

### CHK-11 — every locale has the same copy structure as the default

Recursively walks each locale's `copy` against the default locale's: missing keys,
extra keys, type mismatches, and **array length**.

Remedy: `bring the locale's content file into the same shape as <default>.yaml`

TypeScript already makes a missing key a compile error. What it cannot see is
length: five gates in one language and three in another satisfy the same type, and
the scene renders a short table in silence.

### CHK-14 — translated copy stays within its length budget

Fails when a translated string exceeds `runeCount(default) × expansionFactor`.

Remedy: `shorten the line, or raise this locale's expansionFactor if the layout really does fit`

A proxy for visual overflow, not a proof of it. Shorten first; raise the factor
only after looking at the frame.

### CHK-15 — strings the system prints survive translation verbatim

For every default-locale string containing a declared `printedLiterals` entry,
requires the same literal in the translation.

Remedy: `restore the literal exactly as the system prints it`

### CHK-16 — domain terms are not translated away

For each `glossary` term present in a default-locale prose string, requires it in
the translation. Whole words, case-insensitive, prose only; strings that are
themselves declared printed literals are skipped.

Remedy: `keep the term in the source language, or drop it from the glossary if it really should be translated`

Those bounds are the fix for a check that was previously vacuous: the reference
version stringified the whole copy object including code literals and matched
without word boundaries, so `turn` was satisfied by `returns`.

### CHK-17 — text is normalized so stacked marks render as one glyph

Fails on any `copy` string or narration line that is not in Unicode Normalization
Form C.

Remedy: `normalize the file to NFC`

Decomposed text looks identical in review and depends on the font's mark
attachment on screen. Ten lines of check; the highest value per line in the set.

## Scene shape and timing

### CHK-12 — scene length comes from exactly one source

Fails on a narrated scene declaring `durationInFrames`, and on an unnarrated scene
declaring none.

Remedy: `a narrated scene sets leadFrames/tailFrames only; an unnarrated one sets durationInFrames only`

Allowing both on one scene is how a hand-maintained frame table grows back after
the derivation removed it.

### CHK-13 — no narration begins before its scene is fully opaque

Fails when any narrated scene after the first has `leadFrames < video.transitionFrames`.

Remedy: `raise leadFrames to at least <transitionFrames>, or lower video.transitionFrames`

The cross-fade overlaps two scenes. A line that starts before its own scene is
opaque is spoken over the previous one — audible, and invisible in any single
frame.

### CHK-25 — every scene is long enough for its reveals to separate

Fails on a derived scene duration below `video.minSceneFrames`.

Remedy: `lengthen the line, raise tailFrames, or lower video.minSceneFrames`

Reveals are cued as fractions, so a short scene collapses them onto each other and
the frame flashes through its own content.

### CHK-26 — the finished cut is inside the duration it was commissioned at

Divides each locale's `totalFrames` by `fps` and compares against
`video.targetDuration`. Silent unless that key is present, and silent for any
locale whose timeline is not yet complete.

Remedy: `cut or lengthen the narration, or widen video.targetDuration`

A script drifts longer one clarifying sentence at a time, and nobody notices
until the cut is watched end to end. The session this check comes from asked for
two to three minutes, wrote three and a half, and spent two rewrite rounds
counting words to find out — while the tool already held the number.

Two deliberate limits. It waits for measured audio, because an estimate carries
real error and failing on a length that is not yet true would block work on a
guess; `nv status` shows the estimate against the target from the first draft on,
where being advisory is the right register. And there is no `--force`: the escape
is to widen or drop `targetDuration`, because a flag that waves the gate through
would cost the exit code the one property it has.

## Fonts

### CHK-18 — each locale's font has a glyph for every character it needs

For a **local** font: parses the file and tests every rune of `requiredSample`
against the real cmap. For a **google** font: tests the sample against the
standard ranges of the subsets the config requests — because the mistake there is
forgetting to request the subset at all, after which the browser falls back
silently.

Fails on a missing glyph, an unknown subset name, a font key that is not defined,
an unreadable file, or a locale with **no `requiredSample`** (nothing about its
font can be checked).

Remedy: `choose a face that covers this script — a missing glyph renders as its base letter, with nothing on screen to show it`

Full rationale, the measured codepoint table, and the honest limit: `fonts.md`.

## Provider and spend

### CHK-19 — committed audio came from a provider whose output may ship

Reads the `provider` each manifest records and asks whether it is committable.

Remedy: `re-run with the real provider: nv voiceover -- <locale>`

macOS `say` is not: its length varies with OS version, installed voice and system
speech rate, so a `say`-seeded timeline regenerates differently on another machine
and breaks CHK-01.

### CHK-20 — the declared spend ceiling covers the current script

Estimates the cost of synthesizing every locale at the configured models' rates
and compares against `tts.costCapUsd`.

Remedy: `raise tts.costCapUsd, shorten the script, or synthesize one locale at a time`

The schema already requires a ceiling to exist, so repeating that would be a check
that can never fire. What is worth knowing **before** running synthesis is whether
the script as written would breach it.

## Source discipline

### CHK-21 — scenes learn their length from props only

Fails when a scene module imports `generated/timeline`, `generated/registry` or
`video.config`.

Remedy: `take durationInFrames and leadFrames from SceneProps instead`

A scene that can read the table can also hold a number that contradicts it, and
the validator cannot execute TypeScript.

### CHK-22 — reveals are cued as fractions, so a translation re-times itself

Fails on `at(…, <numeric literal>)` in a scene source, reporting scene and line.

Remedy: `pass durationInFrames as the second argument`

Only the **second** argument is constrained; the first is expected to be a literal
fraction. A blanket ban on numeric literals is unimplementable — scenes carry five
to fourteen legitimately each.

### CHK-23 — the generated timeline loads without a bundler

Fails when `timeline.ts` or `content.ts` imports `remotion`, `react` or
`@remotion/*` as a value. Type-only imports are erased before anything runs and are
allowed.

Remedy: `regenerate with: nv sync`

The timeline is read by tests and tools with no bundler. One value import and the
numbers the whole video depends on sit behind the thing most likely to be broken.

### CHK-24 — every declared scene has a module and every module is declared

Fails on a config entry with no `src/scenes/<Id>.tsx`, and on a `.tsx` in that
directory (not prefixed `_`) that no config entry refers to.

Remedy: `add the scene to video.config.yaml (nv init --scene <Id> scaffolds one), or delete the orphan module`

### CHK-27 — the render scripts target the composition this config declares

Reads `package.json`, and for each of `scripts.render` and `scripts.still` takes
the token immediately after `remotion render` / `remotion still` as the
composition id. Fails when that id is not `video.id` (the default locale's
composition).

Remedy: `run: nv sync`

The id had two homes. `nv sync` writes it into `src/generated/`, while
`package.json` carried a copy typed once by the template — so renaming the
composition left `bun run render` pointing at one that no longer exists, and the
failure arrives after the voiceover has been paid for.

`nv sync` now rewrites that token, and this check catches a copy edited out from
under it. Only the id is claimed: flags, the output path, and any script whose
first token after the subcommand is a flag are left alone, and a `package.json`
that is absent or unreadable passes — it belongs to the JavaScript project, not
to `nv`.

### CHK-34 — package.json installs what the scene kinds in use require

Reads every `src/scenes/<Id>.tsx` the config declares, works out each one's kind
from the packages it imports, and takes the union of what those kinds need. Fails
when `package.json` does not list one of them, or lists it at a different version.

Remedy: `run: nv sync`

A scene's kind lives in exactly one place — its imports — and the packages that
back it live in another, `package.json`. Two surfaces, and when they disagree the
failure lands at bundle time as an unresolved import, after the voiceover has been
paid for. `nv init --scene <Id> --kind flow` writes both halves; this catches a
`package.json` edited out from under them, and a scene whose kind changed by hand.

Reconciliation adds and corrects; it never removes. A `package.json` holds
packages `nv` knows nothing about, and deleting what it does not recognise would
be a destructive answer to a question nobody asked. That is also what keeps this
check and `nv sync` congruent: everything the check reports is what the remedy
fixes, and nothing else moves. A `package.json` that is absent, unreadable, or has
no `dependencies` object passes — it belongs to the JavaScript project, not to
`nv`.

## Reserved ids

Check ids are permanent labels, not positions. `CHK-28` through `CHK-33` are held
for checks that are not written yet, which is why this file lists 28 checks whose
highest id is 34. An id is never reused either — one in a CI log or an old issue
has to keep meaning the single thing it always meant.

## What is deliberately excluded

No check reads pixels, asks a model to judge prose, touches the network, or needs
an API key. Each exclusion is load-bearing, because the exit code has exactly one
property and these would each destroy it.

| Excluded | Why |
| --- | --- |
| **Pixels** — screenshotting frames and diffing them | Output depends on the GPU, the Chrome build, font hinting and antialiasing. The same commit would pass on one machine and fail on another, and a gate that fails at random gets ignored, then removed. Look at stills by eye instead — `bun run still`. |
| **Model judgement** on prose, tone or translation quality | Not reproducible run to run, and not reviewable: two runs on identical files can disagree, so a failure carries no information about what changed. `nv` checks structure, provenance and coverage; a human still reads the script. |
| **Network** — reaching the TTS API, fetching Google's live font metadata | Makes the gate fail on a plane, in a locked-down CI container, and during someone else's outage. It also makes the answer depend on a third party's uptime rather than on the repo. |
| **API keys** | The gate must run on a fresh clone with no credentials, in a bare container, before `bun install`. Needing a key would make "did this pass?" depend on who ran it. |

What that costs: the gate cannot tell you the narration is *good*, that the
Vietnamese is idiomatic, that a caption looks right against the background, or
that a font's glyph is non-empty rather than merely present. Those are the things
to spend human review on — which is the trade, and the reason the mechanical
failures are worth automating out of the way.
