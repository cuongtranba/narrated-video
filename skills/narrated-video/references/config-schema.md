# `video.config.yaml`

Answers: what every field means, what it defaults to, and why it exists.

Derived from `internal/schema/video.schema.json`, which is the single source of
truth — it is validated against (CHK-02), it is what defaults are documented in,
and `nv init` copies it into the project as `video.schema.json` so an editor can
complete the config against the same document the tool validates it with.

Required top-level keys: `kitVersion`, `video`, `locales`, `fonts`, `defaults`,
`scenes`, `tts`. Optional: `theme`, `glossary`, `printedLiterals`. Unknown keys are
rejected everywhere (`additionalProperties: false`), so a typo is a failing check
rather than a setting that silently does nothing.

## Contents

- [`kitVersion`](#kitversion)
- [`video`](#video)
- [`locales`](#locales)
- [`fonts`](#fonts)
- [`theme`](#theme)
- [`defaults`](#defaults)
- [`scenes`](#scenes)
- [`tts`](#tts)
- [`glossary` and `printedLiterals`](#glossary-and-printedliterals)
- [Complete annotated example](#complete-annotated-example)

## `kitVersion`

Integer ≥ 1. The schema generation this file was written against.

## `video`

| field | type | default | notes |
| --- | --- | --- | --- |
| `id` | `^[A-Za-z][A-Za-z0-9-]*$` | required | Base composition id. Non-default locales append `-<locale>`, so the default locale keeps the bare id and the composition anyone renders by habit stays where it was. |
| `width` | integer ≥ 16 | required | |
| `height` | integer ≥ 16 | required | |
| `fps` | integer 1–240 | required | Changing this invalidates every committed frame count; CHK-09 catches it by comparing against the fps stamped in each manifest. |
| `transitionFrames` | integer ≥ 0 | `14` | Cross-fade length, subtracted once per scene boundary from the total. `defaults.leadFrames` must cover it (CHK-13). |
| `minSceneFrames` | integer ≥ 1 | `60` | Floor below which fraction-cued reveals land on top of each other. CHK-25. |
| `out` | string | `"out"` | Output directory. |

## `locales`

`default` is a locale code (`^[a-z]{2}(-[A-Za-z0-9]+)*$`) and must be one of the
codes in `list`. It is the baseline for every structural and budget comparison.

Each entry in `list`:

| field | type | default | notes |
| --- | --- | --- | --- |
| `code` | locale code | required | |
| `label` | non-empty string | required | Human name, for UI. |
| `font` | string | required | Key into `fonts`. Per-locale **because a face that covers latin may silently drop this script's marks** — see `fonts.md`. |
| `charsPerSecond` | number > 0 | `16` | Speaking rate used **only** to estimate timing before audio exists. Measured: en 16.17, vi 15.55. |
| `expansionFactor` | number ≥ 0.1 | `1.3` | Budget multiplier over the default locale's string length (CHK-14). A proxy for overflow, not a proof. The default locale's own factor is `1.0`. |
| `requiredSample` | string | — | Every codepoint here must exist in this locale's font (CHK-18). Include the script's hardest stacked marks. Omitting it **fails** CHK-18: nothing about the font can be checked. |

## `fonts`

A map of name → face. At least one entry. Two shapes.

**Google:**

| field | type | notes |
| --- | --- | --- |
| `kind` | `"google"` | |
| `family` | non-empty string | e.g. `Be Vietnam Pro` |
| `importName` | string | The `@remotion/google-fonts` module name, e.g. `BeVietnamPro`. |
| `subsets` | non-empty array | Only requested subsets get an `@font-face`. Omitting one falls back silently. Known: `latin`, `latin-ext`, `vietnamese`, `cyrillic`, `cyrillic-ext`, `greek`, `greek-ext`. |
| `weights` | array of strings | e.g. `["400", "600", "700"]` |

**Local:**

| field | type | notes |
| --- | --- | --- |
| `kind` | `"local"` | |
| `family` | non-empty string | |
| `files` | non-empty array of `{path, weight?, style?}` | `path` is relative to `public/`. Its real cmap is read — a declared subset is not trusted. `style` is `normal` or `italic`. |

The names here must match the families loaded in `src/fonts.ts`. The config is what
`nv` checks; `src/fonts.ts` is what the browser gets.

## `theme`

A flat map of string → string, emitted verbatim into `src/generated/theme.ts` as
`THEME`. Colour only: the type scale and safe area are layout decisions the
components own, and `theme` holds strings, so putting `safeX: "140"` here would buy
a `parseInt` and no ownership.

OKLCH is the recommended notation because the renderer is Chrome, which parses it
natively — the video quotes your palette rather than an approximation of it.

## `defaults`

| field | type | default | notes |
| --- | --- | --- | --- |
| `leadFrames` | integer ≥ 0 | `14` | Silence at the head, and the frame the `<Audio>` mounts at. Must cover the cross-fade or the line starts under the outgoing scene (CHK-13). |
| `tailFrames` | integer ≥ 0 | `24` | Silence after the voice stops. This is the measured median tail across the reference cut's two locales — see the table in `timing-model.md`. |

## `scenes`

An ordered array, minimum one entry. A bare string is a scene id taking the
defaults; a table overrides them.

```yaml
scenes:
  - Title
  - id: Iteration
    tailFrames: 50
  - id: Chapter
    narrated: false
    durationInFrames: 90
  - Outro
```

| field | type | default | notes |
| --- | --- | --- | --- |
| `id` | `^[A-Z][A-Za-z0-9]*$` | required | Must have a module at `src/scenes/<id>.tsx` (CHK-24). |
| `leadFrames` | integer ≥ 0 | `defaults.leadFrames` | |
| `tailFrames` | integer ≥ 0 | `defaults.tailFrames` | |
| `narrated` | boolean | `true` | |
| `durationInFrames` | integer ≥ 1 | — | **Unnarrated scenes only.** A narrated scene declaring this is CHK-12 — that is how the hand-maintained frame table creeps back. |

`narrated` and `durationInFrames` are deliberately **not** inferred from each
other. Letting a declared duration imply `narrated: false` would silence a scene
without saying so — the exact class of quiet failure this tool exists to make
loud. Stating both is two words; a scene that lost its voice is a re-render to
notice.

An explicit `0` is distinguishable from an absent key, because zero is a legal
frame count.

## `tts`

| field | type | default | notes |
| --- | --- | --- | --- |
| `provider` | `elevenlabs` \| `silence` \| `say` | required | See `tts-providers.md`. |
| `apiKeyEnv` | `^[A-Z][A-Z0-9_]*$` | — | Env var **NAME**. Never the key itself — CHK-03 refuses a literal, in this file and in every tracked file. |
| `costCapUsd` | number > 0 | required | `nv voiceover` estimates before the first request and refuses above this; CHK-20 checks the same arithmetic at validate time. |
| `outputFormat` | string | `"mp3_44100_128"` | |
| `voices` | map of locale → voice | required, ≥ 1 | |

Each voice:

| field | type | notes |
| --- | --- | --- |
| `voiceId` | non-empty string | required |
| `model` | non-empty string | required. **Load-bearing per locale**: `eleven_multilingual_v2` speaks Vietnamese with wrong tones and still returns 200. `nv` refuses a denied *or unlisted* model. |
| `voiceSettings` | map of string → number | Omit **entirely** for a model that does not want it. Settings tuned for one model are not tuning for another, and the models cleared for Vietnamese were cleared with none. |

## `glossary` and `printedLiterals`

Both arrays of non-empty strings, both optional; an empty list makes its check pass
trivially.

- `glossary` — domain terms that must survive translation verbatim (CHK-16). A
  translated term stops being greppable. Matched whole-word, case-insensitive, in
  prose only.
- `printedLiterals` — strings the system itself prints (CHK-15). Translating one
  makes the video contradict the product. Also excluded from glossary matching, so
  a code literal cannot satisfy a term.

## Complete annotated example

```yaml
kitVersion: 1

video:
  id: Explainer            # renders as `Explainer` (en) and `Explainer-vi`
  width: 1920
  height: 1080
  fps: 30
  transitionFrames: 14     # cross-fade; defaults.leadFrames must be >= this
  minSceneFrames: 60       # floor below which fraction cues collide
  out: out

locales:
  default: en
  list:
    - code: en
      label: English
      charsPerSecond: 16.17   # measured, not guessed; estimates only
      expansionFactor: 1.0    # the baseline is its own budget
      font: body
      requiredSample: "The quick brown fox jumps over the lazy dog"
    - code: vi
      label: Tiếng Việt
      charsPerSecond: 15.55
      expansionFactor: 1.35
      font: bodyVi            # a DIFFERENT face — see fonts.md
      requiredSample: "tuyên bố ố ầ ế ấ ư ơ đ ọ"   # the hard stacked marks

fonts:
  body:
    kind: google
    family: Inter
    importName: Inter
    subsets: [latin]
    weights: ["400", "600", "700"]
  bodyVi:
    kind: google
    family: Be Vietnam Pro
    importName: BeVietnamPro
    subsets: [latin, vietnamese]     # forgetting `vietnamese` falls back silently
    weights: ["400", "600", "700"]
  mono:
    kind: google
    family: Roboto Mono
    importName: RobotoMono
    subsets: [latin]
    weights: ["400", "500"]

theme:
  background: oklch(16% 0.01 13)
  foreground: oklch(98% 0.003 13)
  muted: oklch(70% 0.012 13)
  surface: oklch(23% 0.01 13)
  border: oklch(29% 0.008 13)
  accent: oklch(71.2% 0.194 13.428)

defaults:
  leadFrames: 14           # also the frame the <Audio> mounts at
  tailFrames: 24           # measured median across the reference cut

scenes:
  - Title
  - id: Iteration
    tailFrames: 50         # holds in silence past the last word — that IS the point
  - id: Chapter
    narrated: false        # both keys stated; neither is inferred
    durationInFrames: 90
  - Outro

tts:
  provider: silence        # switch to elevenlabs when the script has settled
  apiKeyEnv: ELEVENLABS_API_KEY   # a NAME. never the key
  costCapUsd: 2.0
  outputFormat: mp3_44100_128
  voices:
    en:
      voiceId: SAz9YHcvj6GT2YYXdXww
      model: eleven_multilingual_v2
      voiceSettings:
        stability: 0.55
        similarity_boost: 0.75
        style: 0.15
    vi:
      voiceId: SAz9YHcvj6GT2YYXdXww
      model: eleven_turbo_v2_5      # multilingual_v2 is DENIED for vi
      # no voiceSettings at all — this model was cleared by ear without them

glossary: [loop, subagent, worktree]

printedLiterals:
  - "GOAL MET"
  - "session_token"
```
