# Narrated video

A Remotion project whose timing comes from measured audio rather than from a
table someone maintains by hand.

```bash
bun install
bun run studio     # every scene is also its own composition
bun run render     # → out/explainer.mp4
```

## How a change flows

Config and content are data; TypeScript is generated from them, in one
direction only:

```
video.config.yaml  ─┐
content/<locale>.yaml ├─→ nv sync ─→ src/generated/* ─→ components
measured audio     ─┘
```

Nothing flows back. **Every file under `src/generated/` is overwritten by
`nv sync`** — edit the YAML instead, and the type arrives with the value.

| I want to… | Edit |
| --- | --- |
| change a spoken line, or an on-screen string | `content/<locale>.yaml` |
| add or reorder a scene | `scenes:` in `video.config.yaml`, then `nv sync` |
| add a language | `locales.list` + a new `content/<code>.yaml` |
| change the palette | `theme:` in `video.config.yaml` |
| change how long a scene runs | record the narration — that IS the length |

Then: `nv sync`, `nv validate`, `bun run typecheck`.

## Scene lengths are derived, not chosen

A scene lasts `leadFrames + narration + tailFrames`, with the cross-fade repaid
once per boundary. Only the two pads are a person's decision, and they are set
once in `defaults`. A narrated scene may not declare `durationInFrames`; that
refusal is what stops the hand-kept frame table from growing back.

Before any audio exists, lengths are estimated from character counts —
±15%, good enough to review layout. Anything estimated wears the **UNVOICED**
badge, in Studio and in a real render alike, with no way to switch it off. Run
`nv voiceover` and it goes away.

## Writing a scene

Copy `src/scenes/_template.tsx`, or `nv init --scene <Id>`. Three rules:

- Export exactly one symbol, named `Scene`. The generated registry imports it
  as `import { Scene as <Id> }`.
- Cue reveals as **fractions** of `durationInFrames` via `at()` — never frame
  counts. That is what lets a translation re-time itself instead of carrying its
  own table of cues.
- Read strings from `useCopy()`, never from a prop. A prop can be forgotten one
  level down, and that caption renders in English inside every other cut.

Scene modules hold no number about their own length, and never import
`src/generated/timeline`, `src/generated/registry`, or the config.

## Narration

`tts.provider: silence` is the default: correctly-sized silent audio, no
account, no key, no network — enough to see real pacing. Switch to
`elevenlabs`, put the key in the env var named by `tts.apiKeyEnv` (see
`env.example`), and run `nv voiceover`.

The `model` is per locale and it fails silently: a multilingual model can speak
a tonal language with the tones wrong and still return a clean 200 and a
plausible duration. Judge a new language by ear, not by the request succeeding.

Audio is committed under `public/voiceover/<locale>/`, so a clone with no key
renders the finished cut.
