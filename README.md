# narrated-video

A skill that ships a prebuilt gate for narrated explainer videos.

`nv` is a single static Go binary. It scaffolds a [Remotion](https://remotion.dev)
project, derives every scene's length from **measured** audio, synthesizes the
narration, and runs 25 checks whose exit code is the contract. The project it
writes carries no build tooling of its own — no `scripts/`, no `ajv`, no YAML
library — so a broken `node_modules` can change what renders but cannot change
what the gate says.

## Install

```bash
npx skills add cuongtranba/narrated-video
export PATH="$HOME/.agents/skills/narrated-video/bin:$PATH"
```

One clone. Binaries arrive executable; nothing is built or downloaded at install
time.

## Quickstart

```bash
nv init my-video && cd my-video     # wrote 28 files
nv validate                         # exit 1 — CHK-05 and CHK-06 only
nv voiceover                        # provider `silence`: no API key, no network
nv validate                         # exit 0 — "25 checks passed"
bun install && bun run studio
```

Real output from `nv voiceover` on a fresh scaffold:

```
60 characters across 1 locale(s), estimated $0.00 (cap $2.00)

── en — eleven_multilingual_v2 via silence ──
Title              2.22s     67f
Outro              1.49s     45f

regenerating:
  src/generated/timeline.ts
  ...
```

Commands: `init [dir]` (`--scene <Id>`), `sync`, `validate` (`--json`),
`voiceover [locale…]` (`--force`), `version`. Run any of them from anywhere inside
a project — the root is found by walking up, like `git`.

## Why

This exists because of one measured session. Building an eight-scene, two-language
explainer by hand cost **374 tool calls, 111 minutes and $38.32**, and four of its
five substantive turns existed only because the previous turn shipped something
wrong:

- **The frame table was hand-rewritten seven times** — once per text-to-speech run.
  Synthesis is not deterministic: identical text re-synthesized shifted lengths by
  up to 14 frames. Every rewrite was a human recomputing `narration + pad`.
  → `nv` derives it, and byte-compares a fresh derivation against what is on disk.
- **The wrong TTS model shipped.** `eleven_multilingual_v2` speaks Vietnamese with
  the tones wrong while returning HTTP 200 and a plausible duration. Undetectable
  without a native ear.
  → per-locale model policy; a denied *or unlisted* model is refused.
- **The font silently dropped diacritics.** `tuyên bố` rendered as `tuyên bô` —
  no missing-glyph box, caught only by cropping a full-resolution still. The face
  contains `ọ` (inside Google's `vietnamese` range, so every coarse test passes)
  and does not contain `ố`, `ầ`, `ế`, `ấ`, `ư`, `ơ`.
  → `nv` reads the font's real `cmap`, from its bytes.
- **The API key was pasted inline about a dozen times** and never rotated.
  → keys are referenced by env-var name, and every tracked file is scanned for key
  shapes.
- **~29 tool calls of pure scaffold plumbing**, most of it undoing a template's
  defaults. → one `nv init`.

Plus 28 `tsc` runs, 15 `eslint` runs, 14 renders and 20 stills read back by eye.

## How it fits together

```
video.config.yaml ──┐
                    ├──► nv sync ──► src/generated/timeline.ts   (data only)
public/voiceover/  ─┘            └──► src/generated/registry.ts  (component bindings)
   <locale>/manifest.json
                    └──► nv validate ──► exit 0 | exit 1 + remedies
```

Unidirectional: config and measurements → generated code → Remotion. Nothing flows
back. `duration = leadFrames + narration + tailFrames`, with the cross-fade repaid
once per boundary; reveals inside a scene are cued as fractions of it, so a
translation re-times itself.

## Repo layout

```
cmd/nv/                     the CLI
internal/config/            schema-validated config, single source of defaults
internal/timing/            the derivation; tested against the reference cut's 16 numbers
internal/gen/               the TypeScript codegen
internal/tts/               provider interface + elevenlabs, silence, say
internal/mp3/               MPEG frame-header duration measurement
internal/fonts/             woff2/sfnt cmap reader, unicode-range parser, NFC
internal/checks/            the 25 checks
internal/scaffold/          nv init
internal/schema/            video.schema.json
kit/                        //go:embed — the Remotion project template
skills/narrated-video/      what lands in a user's agent directory
  SKILL.md  references/*.md
  bin/nv                    POSIX shim
  bin/{darwin,linux}-{arm64,amd64}/nv
```

`kit/` is embedded in the binary rather than shipped as loose files, so `nv init`
is self-sufficient and the installed skill stays at four binaries plus prose.

## Build from source

Requires Go 1.26.6.

```bash
make build       # → bin/nv
make binaries    # cross-compile all four targets into skills/narrated-video/bin/
make check       # gofmt -l, go vet, go test
make all         # check + binaries
```

Built `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`, which with a pinned toolchain is
byte-reproducible — that is what lets a rebuild assert the committed binaries match
their source. The binaries carry **no version stamp**, deliberately: writing one
would change the artifact being compared. `nv version` prints a content hash
instead.

## Distribution caveat

This skill **cannot be redistributed inside a skills.sh pack** — packs exclude
binaries and files over 2 MB, and each `nv` target is ~8 MB. Install via
`npx skills add cuongtranba/narrated-video`, which clones.

Two related constraints for anyone forking this: the `skills` CLI force-disables
Git LFS during clone (LFS-tracked binaries would arrive as ~130-byte pointer
stubs, so binaries are committed directly and `.gitattributes` must not route them
through LFS), and the CLI runs no lifecycle hooks, so there is no postinstall in
which to build or download. The binary has to already be there.

Windows is not supported in v1. macOS and Linux, arm64 and amd64.

## Documentation

`skills/narrated-video/SKILL.md` routes; the detail lives in `references/`:
`timing-model.md`, `scene-registry.md`, `tts-providers.md`, `localization.md`,
`fonts.md`, `config-schema.md`, `validate-checks.md`, `troubleshooting.md`.

## License

MIT — see [LICENSE](LICENSE).
