# TTS providers

Answers: which provider to use, which model is safe for which language, why an
unlisted model is refused, and how the key stays out of the repo.

## Contents

- [The interface](#the-interface)
- [The three bundled providers](#the-three-bundled-providers)
- [Model policy](#model-policy)
- [Correction: the remotion-best-practices skill is wrong here](#correction-the-remotion-best-practices-skill-is-wrong-here)
- [The key](#the-key)
- [Running it](#running-it)
- [The manifest](#the-manifest)
- [Cost](#cost)
- [Adding a provider](#adding-a-provider)

## The interface

```go
type Provider interface {
	ID() string
	RequiredEnv() []string // env var NAMES only
	Deterministic() bool   // same text => same bytes on any machine
	Committable() bool     // may its output ship as the narration?
	Policy() ModelPolicy
	PricePer1kChars(model string) float64
	Synthesize(ctx context.Context, r Request) error
}
```

`Synthesize` writes a file and **returns no duration**, by design. The driver
measures the written bytes with `nv`'s own MPEG frame-header walker
(`internal/mp3`), uniformly for every provider. A provider that reports its own
duration can be wrong, and that number is the timing contract for the entire
video — every scene length, every fraction cue, the composition's
`durationInFrames`. The audio is the contract; a JSON field about the audio is not.

Audio is published through a temp file plus a rename, so a measured file is never
a partial one. A truncated read would silently shorten a scene.

## The three bundled providers

| provider | deterministic | committable | role |
| --- | --- | --- | --- |
| `elevenlabs` | no | yes | production |
| `silence` | **yes** | yes | **CI and no-key default** |
| `say` (macOS) | no | **no** | hear local pacing only |

**`silence`** writes a valid CBR MP3 by repeating one pre-encoded silent frame
(418 bytes: `FF FB 92 C0` then zeros — MPEG-1 Layer III, 128 kbps, 44100 Hz,
mono, padded; zeroed side info declares no Huffman data, which every decoder
renders as silence). Length is `runeCount / charsPerSecond`. Byte-identical on
every machine, zero network, no account. This is what makes `nv init` → `nv
voiceover` → `nv validate` exit 0 with no credentials at all.

**`say`** drives macOS's built-in synthesizer and re-encodes with `ffmpeg` or
`lame`. It is the zero-setup way to *hear* a script read back. It is disqualified
as a committable source not merely for being macOS-only: its length varies with
the OS version, the installed voice and the system speech rate, so a `say`-seeded
timeline regenerates differently on a colleague's laptop and breaks the CHK-01
byte-compare. **CHK-19** reads the provider recorded in the manifest and refuses
to let `say` output ship.

**`elevenlabs`** posts to `/v1/text-to-speech/<voiceId>?output_format=<format>`
with `xi-api-key`. Non-deterministic: the same text resynthesized returns
different bytes and a different length, which is exactly why the frame table is
re-derived from measurement on every run rather than maintained.

## Model policy

Each provider carries a per-locale policy of models a native speaker's ear has
cleared, plus explicit denials that travel with the observation that disqualified
them:

```go
Deny: map[string][]DeniedModel{
	"vi": {{
		Model: "eleven_multilingual_v2",
		Why: "speaks Vietnamese with wrong tones; returns 200 with a plausible duration, " +
			"so nothing but listening catches it. Use eleven_turbo_v2_5 with no voice_settings.",
	}},
},
Allow: map[string][]string{
	"vi": {"eleven_turbo_v2_5"},
	"en": {"eleven_multilingual_v2", "eleven_turbo_v2_5"},
},
```

**An unlisted model fails too**, with a distinct error (`ErrModelUnlisted` vs
`ErrModelDenied`). This is the part people try to relax, so: a model that has
never been judged by ear for a language must not default into use merely because
the request would succeed. The failure class here is invisible to the machine —
correct HTTP status, plausible duration, plausible waveform, wrong language. The
allow-list is the *only* record that a human listened.

Adding a language therefore means: synthesize one line, **listen to it with
someone who speaks the language**, and then add the model to `Allow` in
`internal/tts/elevenlabs.go`. Not the other way round.

`voiceSettings` is omitted from the request entirely when absent from the config,
never defaulted. Settings tuned against one model mistune another, and the models
cleared for Vietnamese were cleared **with no settings at all**.

Two checks enforce the policy, and they are not redundant:

- **CHK-04** reads the config — what was *intended*.
- **CHK-08** reads the manifest — what was *actually used*. An
  `ELEVENLABS_MODEL_ID=` style override, or a config edited after the fact, would
  sail past CHK-04 alone, and the resulting mp3 is indistinguishable from a good
  one without listening.

## Correction: the remotion-best-practices skill is wrong here

If the `remotion-best-practices` skill is installed on this machine, its
`remotion-markup/voiceover.md` hardcodes:

```js
model_id: "eleven_multilingual_v2",
```

That is the exact model this project denies for Vietnamese — the one that returns
200 with a plausible duration while speaking the tones wrong. Use
`remotion-best-practices` for Remotion API questions outside this kit; do not take
its model id. `nv` will refuse it for `vi` regardless, which is the point.

## The key

`tts.apiKeyEnv` holds an environment variable **name**, matching
`^[A-Z][A-Z0-9_]*$`. `nv voiceover` reads `os.Getenv(name)` at the call site,
passes it by value, and it is never logged, never written to the config, and never
lands in a generated file or a manifest.

**CHK-03** scans the config, every content file, and every git-tracked text file
under 1 MiB for credential shapes: `sk_…`, `sk-…`, `xi-…`, `AIza…`, `ghp_…`. It is
deliberately wider than the config because the project this came from leaked its
key through roughly a dozen shell commands, not through YAML. A key in a tracked
file is in the history from the moment it lands, and rotating it is the only fix.

Copy `.env.example` to `.env` (gitignored) and fill it in there.

## Running it

```bash
nv voiceover            # every locale
nv voiceover en         # one locale — others' measurements are untouched
nv voiceover en vi
nv voiceover --force    # proceed past the declared spend ceiling
```

Real output from a fresh scaffold on the `silence` provider:

```
60 characters across 1 locale(s), estimated $0.00 (cap $2.00)

── en — eleven_multilingual_v2 via silence ──
Title              2.22s     67f
Outro              1.49s     45f

regenerating:
  src/generated/timeline.ts
  src/generated/registry.ts
  src/generated/theme.ts
  src/generated/content.ts
```

It re-syncs automatically. Measurements just changed, so the generated timeline is
stale by definition; leaving that to the operator is how a project ends up with
scenes timed to audio it no longer has.

Re-synthesizing one locale never disturbs another's measurements — each is timed
from its own audio, in its own `public/voiceover/<locale>/manifest.json`.

## The manifest

`public/voiceover/<locale>/manifest.json`, written per locale:

```json
{
  "fps": 30,
  "provider": "silence",
  "scenes": {
    "Title": {
      "sha256": "…",
      "seconds": 2.22,
      "frames": 67,
      "provider": "silence",
      "model": "eleven_multilingual_v2",
      "voiceId": "SAz9YHcvj6GT2YYXdXww",
      "bytes": 35530
    }
  }
}
```

Every field is read by a check:

| field | check | catches |
| --- | --- | --- |
| `sha256` of the spoken line | CHK-07 | a line edited after its take was recorded — the one drift a render cannot reveal, because the old take still plays |
| `frames` | CHK-05 | a scene still on an estimate |
| `provider` (per scene and per file) | CHK-08, CHK-19 | audio from a provider whose output must not ship |
| `model` | CHK-08 | audio actually synthesized with a denied model |
| `fps` | CHK-09 | frame counts measured at a rate the video no longer uses |

Commit `public/voiceover/`. Audio in the repo is what lets someone with no API key
render the finished cut rather than a silent draft.

## Cost

`tts.costCapUsd` is required by the schema. `nv voiceover` estimates the whole run
**before the first request** — billing is per character and a loop over locales
multiplies it quietly — and refuses above the cap unless `--force`. ElevenLabs
prices per 1000 characters: `eleven_turbo_v2_5` and `eleven_flash_v2_5` $0.15,
`eleven_multilingual_v2` $0.30; an unknown model bills at the highest known tier,
because an estimate that is too low is the one that surprises someone.

**CHK-20** does the same arithmetic at validate time, so a script that has grown
past its ceiling is a failing check rather than a refusal discovered mid-run.

## Adding a provider

One file in `internal/tts/`, one row in the registry map, one section here.
Checklist:

- `Deterministic()` and `Committable()` are separate questions. `silence` is both;
  `elevenlabs` is committable and not deterministic; `say` is neither.
- `Policy()` — if output does not vary by model, clear everything with
  `Allow: {"*": {"*"}}`. Otherwise list only what an ear has cleared.
- `Synthesize` writes `r.OutPath` via `writeOutput` and returns no duration.
- `RequiredEnv()` returns variable **names**.
