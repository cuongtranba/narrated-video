# Scenes and the registry

Answers: what a scene module is allowed to contain, how scenes are bound into the
video, and how to add, reorder or remove one.

## The contract

A scene file is `src/scenes/<Id>.tsx` and exports **exactly one symbol, named
`Scene`**, of type `SceneComponent`:

```ts
export interface SceneProps {
  readonly durationInFrames: number
  readonly leadFrames: number
}
export type SceneComponent = React.FC<SceneProps>
```

That is the whole interface. `durationInFrames` is the scene's full length, lead
and tail included; `leadFrames` is the silence before the voice starts.

The export name is load-bearing: `src/generated/registry.ts` is written as
`import { Scene as <Id> } from "../scenes/<Id>"`. A renamed or default export is a
compile error in a generated file — which is a better failure than an undefined
component at frame 240.

## No numbers about time live in a scene module

A scene is **told** how long it lasts. It never looks its own length up, and it
never carries a fallback.

Two rules enforce this, and both exist because the alternative is two surfaces
that can disagree while every frame still renders:

- **CHK-21** — a scene module may not import `generated/timeline`,
  `generated/registry` or `video.config`. The validator reads the config and
  cannot execute TypeScript, so a number inside a module is a number nothing
  checks. The one that lies is always the one nobody is looking at.
- **CHK-22** — the second argument to `at()` may not be a numeric literal.
  `at(0.4, 1209)` pins a cue to one language's audio; the translation then drifts
  away from the sentence explaining it, silently, because every frame still
  renders. Write `at(0.4, durationInFrames)`.

CHK-22 constrains the second argument only. A blanket ban on numeric literals in
scene sources is unimplementable — scenes legitimately carry five to fourteen of
them each (`maxWidth: 1400`, `gap: 34`, `rise={26}`) — so a literal hunt is a
false-positive machine that misses the actual sin, which lives in argument
position.

## Anatomy of a scene

From `src/scenes/Title.tsx`:

```tsx
const CUE = { subheading: 0.34 }

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => {
  const { title } = useCopy()

  return (
    <Stage name="Title">
      {/* At 0, so the title has settled by the time the first word lands. */}
      <Reveal name="Heading" at={0}>
        <h1>{title.heading}</h1>
      </Reveal>

      {/* Draws itself in on the first word — the lead is exactly that cue. */}
      <Rule at={leadFrames} width={420} color={THEME.accent} />

      <Reveal name="Subheading" at={at(CUE.subheading, durationInFrames)}>
        <p>{title.subheading}</p>
      </Reveal>
    </Stage>
  )
}
```

Three habits worth copying:

- **Cues collected in one `CUE` object** at the top. The scene then reads as a
  score — a column of fractions — instead of forty copies of the same
  `interpolate`.
- **Strings from `useCopy()`, never from a prop.** Language is ambient
  (`CopyProvider` sets it once per composition). A `copy` prop would have to be
  forwarded by every component between the composition and a caption, and the day
  one of them forgets, that caption renders in English inside every other cut.
- **Layout through `Stage`, motion through `Reveal`.** One entrance curve for the
  whole video, one safe area, one heading rhythm.

Available from `src/components/primitives.tsx`: `Reveal` (the one entrance — rise
and fade on the shared ease), `Rule`, `SAFE`, `SIZE`, `RADIUS`, `EASE_OUT`,
`HAIRLINE`. Colours come from `THEME` (`src/generated/theme.ts`, generated from
`theme:` in the config); the type scale and safe area are layout decisions the
components own, not config.

## The generated registry

`nv sync` writes `src/generated/registry.ts`:

```ts
import { Scene as Title } from "../scenes/Title"
import { Scene as Outro } from "../scenes/Outro"

export const SCENE_ORDER = ["Title", "Outro"] as const
export type SceneId = (typeof SCENE_ORDER)[number]

export const SCENE_COMPONENTS: Readonly<Record<SceneId, SceneComponent>> = {
  Title,
  Outro,
}
```

Order comes from `scenes:` in `video.config.yaml`. `src/Video.tsx` joins the
registry to the timeline **by id**, not by position, so neither file has to know
the other's ordering, and a scene the timeline has not heard of is dropped rather
than crashing a render halfway through.

### The module-scope identity trap

Components are bound at **module scope** in the generated registry, and the
standalone scene wrappers in `Root.tsx` are built at module scope too:

```ts
const STANDALONE_SCENES = scheduleFor(DEFAULT_LOCALE).map(({ id, Component, timing }) => {
  const Standalone: React.FC = () => (/* … */)
  return { id, Standalone, durationInFrames: timing.durationInFrames }
})
```

A component identity created **during render** is a different type on every frame.
React unmounts and remounts the whole subtree under it, every frame — state,
`delayRender` handles and any entrance animation reset each time. The reference
project did this inside its `RemotionRoot` body. Keep every component identity
above the render function; if you find yourself writing `const X = () => …` inside
a component, hoist it.

## Standalone per-scene compositions

Every scene is registered twice: once inside each locale's cut, and once on its
own inside a `Scenes` folder in Studio. Double-clicking a sequence in the timeline
opens that scene in isolation — the difference between scrubbing a two-minute
timeline to reach beat seven and just opening beat seven.

Two deliberate limits:

- Only the **default locale** gets standalone compositions. They exist for
  checking layout, and one entry per scene per language would bury the
  compositions anyone actually renders.
- The standalone is passed the same `leadFrames` it gets in the cut. A standalone
  that started its reveals at 0 would look right on its own and land early
  everywhere else.

## Adding a scene

```bash
nv init --scene=Middle
```

Note the `=`. `nv init --scene Middle` (with a space) is parsed as `nv init` into
a directory called `Middle` and scaffolds a whole project there — see
`troubleshooting.md`.

That command writes `src/scenes/Middle.tsx` from `_template.tsx` and appends
`- Middle` to `scenes:` in the config. Then:

1. Add its narration line to every `content/<locale>.yaml` under `narration:`,
   keyed by the scene id, and any on-screen strings under `copy:`.
2. `nv voiceover` (or `nv sync` if the scene is unnarrated).
3. `nv validate`.

Until step 1 you will see CHK-10 (no narration line) and CHK-25 (a scene with no
line estimates to 0 narration frames, so its 38-frame length is under the
60-frame floor). Both are correct and both clear on their own.

`--scene=<Id>` appends to the end of `scenes:`. Reorder by moving the line in the
YAML — that is the whole operation, and `nv sync` re-derives the registry, the
order and every composition from it.

## Removing a scene

Delete its line from `scenes:`, delete `src/scenes/<Id>.tsx`, delete its
`narration:` key and its `copy:` section from every content file, and delete
`public/voiceover/<locale>/<Id>.mp3`. CHK-24 fails on either half done alone: a
config entry with no module, or a module nothing refers to.

Scene ids are PascalCase letters and digits (`^[A-Z][A-Za-z0-9]*$`). Files
beginning `_` are templates and are not treated as scenes.
