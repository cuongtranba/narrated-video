# Troubleshooting

Answers: what `nv init` replaces, the traps in getting `nv` onto a machine, how
big committed audio gets, and the errors that actually come up.

## Contents

- [The 29 scaffold steps this replaces](#the-29-scaffold-steps-this-replaces)
- [`nv` CLI gotchas](#nv-cli-gotchas)
- [Distribution traps](#distribution-traps)
- [Committed audio size](#committed-audio-size)
- [Common Remotion errors](#common-remotion-errors)
- [Common `nv` errors](#common-nv-errors)

## The 29 scaffold steps this replaces

`nv init` writes 29 files plus the schema in one command. Assembled by hand, in
the session this tool came from, that was roughly 29 tool calls:

- `create-video`, then **undoing** its tailwind template
- four separate `remotion add` calls (transitions, media, fonts, google-fonts)
- `remotion.config.ts` rewritten
- `tsconfig.json` edited three times
- `package.json` scripts edited three times
- copying font files around
- probing which google-fonts subsets existed

None of that is a decision anyone wants to make twice. If you find yourself
reaching for `create-video`, `remotion add`, a hand-written `remotion.config.ts`,
or a `scripts/` directory with `ajv` and `@remotion/media-parser` in it — stop.
`nv init` already did it, pinned and consistent.

## `nv` CLI gotchas

**`--scene` takes either spelling.** `nv init --scene Middle` and
`nv init --scene=Middle` both add a scene. If you are on an older build where the
space form scaffolded a whole project into `./Middle` instead, delete that
directory and use `--scene=Middle`.

**`--kind` picks the template a scene starts from.** `text` (the default), `flow`
or `space`; see `scene-registry.md`. It also adds that kind's npm packages to
`package.json`, so run `bun install` when the command says it added any. An
unknown kind is refused before anything is written.

**`--` in remedy text is harmless.** Several remedies print
`nv voiceover -- <locale>`. `nv voiceover en` works identically — bare `--` is
skipped by the argument parser.

**Exit codes.** `0` pass, `1` a failed check or an error, `2` a usage error
(no command, or an unknown one).

**Run from anywhere.** All commands walk up from the current directory looking for
`video.config.yaml`, like `git`. There is no `--root` flag and none is needed.

**`nv init` refuses a directory that already holds a project** (`. already holds a
project`). It does not merge or upgrade.

**`nv version`** prints a content hash of the binary, not a release tag. The
binaries are committed and rebuilt byte-for-byte from source, so stamping a
version into them would make that comparison impossible: writing the stamp changes
the artifact being compared.

## Distribution traps

Three properties of the `skills` CLI, each of which would silently break a skill
that ships a binary:

**Git LFS is force-disabled during clone.** `npx skills add` clones with
`filter.lfs.smudge=` and friends, so an LFS-tracked binary arrives as a ~130-byte
pointer stub — executable, present, and completely wrong. Binaries must be
committed **directly**, and `.gitattributes` must not route them through LFS. If
`nv` fails with a shell syntax error or "cannot execute binary file", check its
size first: 130 bytes means an LFS stub.

**There are no lifecycle hooks.** The CLI never executes anything from a skill.
There is no postinstall to build in, no download step, no compile-on-first-use.
The binary must already be in the repo. This is why all four targets are committed.

**It cannot be redistributed inside a skills.sh pack.** Packs exclude binaries and
files over 2 MB; `nv` is ~8 MB per target. The install path is
`npx skills add cuongtranba/narrated-video`, which clones. (The blob fast-path is
UTF-8 text only and would corrupt a binary anyway, but it is owner-allowlisted, so
this repo always takes the clone path.)

**Executable bit.** The installer preserves `mode & 0o777` on copy, so binaries
arrive at `0755`. That is an implementation detail read out of the CLI's source,
not a documented contract — if `nv` ever installs non-executable, `chmod +x` the
`bin/` tree and file an issue.

**Windows is not supported.** The shim covers `darwin-{arm64,amd64}` and
`linux-{amd64,arm64}` and says so plainly rather than failing obscurely. Build
from source with `make build` if you need another target.

## Committed audio size

`public/voiceover/` is committed on purpose: audio in the repo is what lets someone
with no API key render the finished cut rather than a silent draft.

Rough sizes at `mp3_44100_128` (~16 KB per second of speech):

| Video | Per locale | Two locales |
| --- | --- | --- |
| 90 s, 8 scenes | ~1.5 MB | ~3 MB |
| 5 min, 20 scenes | ~5 MB | ~10 MB |
| 15 min, 60 scenes | ~15 MB | ~30 MB |

Under ~20 MB total, commit directly and do not think about it. Git handles it
fine, and every clone gets a working render.

Above that, consider `git-lfs` **in your video project** — that is a different
decision from this skill's own repo, where LFS is forbidden because the `skills`
CLI disables the smudge filter. Your project is cloned by `git clone`, which
resolves LFS normally. If you do use LFS, note that a shallow or filtered clone may
skip the audio, which surfaces as CHK-06 (`audio-present`) rather than as a
mysterious silent render — that is what the check's separate existence buys.

Lowering `tts.outputFormat` (e.g. `mp3_44100_64`) halves the size. Re-run
`nv voiceover` afterwards: the frame counts are measured from the actual files.

## Common Remotion errors

**A composition disappeared from Studio.** Almost always `src/generated/` is stale
or was hand-edited — check `nv validate` for CHK-01 and run `nv sync`. Compositions
are derived from `LOCALES × TIMELINE`, so a locale missing from the generated
timeline is a locale missing from the list.

**A scene remounts every frame; animations restart; state resets.** A component
identity was created inside a render function. Hoist it to module scope. See the
identity trap section in `scene-registry.md`.

**The first frames render in a fallback font.** Font loading was not wrapped in
`delayRender`/`continueRender`. The kit's `src/fonts.ts` does this; do not
"simplify" it into a bare `void Promise.all(...)`.

**The model (or texture, or HDR environment) is missing from some frames.** A 3D
asset is being loaded without `delayRender`/`continueRender`. Remotion captures
frames synchronously; any load that begins after capture produces a frame where the
asset has not arrived yet, and how many frames are affected depends on disk cache and
machine load — so the failure can pass locally and fail in CI, or the reverse. CHK-35
catches this in source. Fix: load via `staticFile("asset.glb")` and gate the capture:

```tsx
const handle = delayRender()
useGLTF(staticFile("model.glb"), () => continueRender(handle), () => continueRender(handle))
```

The error-path call to `continueRender` is required: if the load fails and the handle
is never released, the render process hangs instead of failing fast.

**The render hangs and never finishes.** A `delayRender()` handle was created but
`continueRender()` is never called on the error path — the load failed silently and
the process is waiting forever. CHK-35 flags a source that calls `delayRender` with
no `continueRender` at all; a handle released only in the success callback is not
caught by source analysis, but can be confirmed by watching for `Error: delayRender
was called but continueRender` in the Remotion console. Fix: call `continueRender(handle)`
in both success and error callbacks — see `references/3d.md`.

**`Cannot find module '../generated/…'`** on a fresh clone — run `nv sync`. The
generated files are committed, so this usually means they were gitignored by
accident; the kit's `.gitignore` deliberately does *not* ignore them.

**A missing asset aborts the render.** The `<Audio>` is gated on `hasAudio` (does
the mp3 exist on disk), not on whether a narration string exists. If you changed
that gate, change it back — a written script with no audio yet is the normal state
of a fresh clone with no API key.

**`bun run render` renders the wrong composition.** The `render` script targets the
default-locale id (`remotion render Explainer out/explainer.mp4`). For another
locale: `bunx remotion render Explainer-vi out/explainer-vi.mp4`.

Renaming `video.id` used to leave that script behind, and the break surfaced only
after the voiceover had been paid for. `nv sync` now rewrites the id in
`scripts.render` and `scripts.still`, and CHK-27 fails if a copy is edited out
from under it. Only the id is claimed — flags and the output path are yours, and
a script whose first token after the subcommand is a flag is left alone entirely.

**Two renders of the same project produce different output.** Remotion captures
frames concurrently in separate browser tabs. If a scene uses wall-clock time or
nondeterministic values, two tabs will disagree — and a single-frame spot-check
looks fine. Run `nv validate` and look for **CHK-28**. Banned calls:

| Call | Deterministic replacement |
| --- | --- |
| `useFrame` (react-three-fiber) | `useCurrentFrame()` from `remotion` |
| `Date.now()` / `new Date()` / `performance.now()` | `useCurrentFrame()` |
| `Math.random()` | `random()` from `remotion` (seeded by frame) |
| `setTimeout` / `setInterval` / `requestAnimationFrame` | `useCurrentFrame()` |

**Type errors in `src/fonts.ts` after adding a locale.** Intended.
`bodyFamilyFor` is keyed by `Locale`, so a new language stops the build until a
face is named for it.

## Common `nv` errors

| Message | Cause | Fix |
| --- | --- | --- |
| `no video.config.yaml found in … or any parent directory` | not inside a project | `cd` into it, or `nv init <dir>` |
| `<dir> already holds a project` | `nv init` onto an existing project | pick another directory |
| `no API key (set ELEVENLABS_API_KEY)` | provider is `elevenlabs`, env var unset | export it, or use `tts.provider: silence` |
| `model denied for locale` | e.g. `eleven_multilingual_v2` for `vi` | use a cleared model — `tts-providers.md` |
| `model not cleared for locale` | model nobody has judged by ear for that language | listen to it, then add it to the provider's `Allow` |
| `estimated $X exceeds tts.costCapUsd of $Y` | script grew past the ceiling | raise the cap, or `--force` |
| `refusing to write empty audio to …` | provider returned nothing | check the API response; usually a bad voice id |
| `mp3: no valid MPEG audio frame found` | the written file is not MPEG audio | usually an API error body saved as `.mp3`; delete it and re-run |
| `unknown locale "xx" — the project declares [en vi]` | typo in a `nv voiceover` argument | use a declared code |
| `"…" is not a scene id — use PascalCase letters and digits` | `nv init --scene my-scene` | `--scene MyScene` |
| `unknown scene kind "…" — valid kinds: text, flow, space` | a typo or an invented `--kind` | use one of the three; nothing was written |
| `needs an mp3 encoder on PATH, found neither ffmpeg nor lame` | `say` provider without an encoder | `brew install ffmpeg` |
| `only available on macOS (running linux)` | `say` provider off macOS | use `silence` or `elevenlabs` |
| `fonts: … is woff1` / `is a font collection` | unsupported local font container | convert to woff2, ttf or a single-face otf |

## When a check fails and the fix is not obvious

Read the check's entry in `validate-checks.md`. Every one of the 25 exists because
something shipped wrong, and the entry says what — which is usually enough to tell
whether the failure is the check being pedantic or the check being right.

It is almost always the check being right. Do not weaken a check to make it pass.
