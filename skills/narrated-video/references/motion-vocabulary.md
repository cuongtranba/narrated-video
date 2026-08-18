# Motion vocabulary

Five animation primitives in `kit/src/components/motion.tsx` that go beyond the
single entrance `Reveal` provides. Each is a pure function of `useCurrentFrame()`;
none runs its own timer or observes the DOM.

Frame math lives in `kit/src/motion-math.ts` and is tested independently of React.

## Stagger

Children enter in order, `step` frames apart.

```tsx
<Stagger name="Steps" at={at(CUE.steps, durationInFrames)} step={4} durationInFrames={durationInFrames}>
  <div>First</div>
  <div>Second</div>
  <div>Third</div>
</Stagger>
```

| Prop | Type | Description |
| --- | --- | --- |
| `name` | `string` | Studio label |
| `at` | `number` | Frame at which the first child enters |
| `step` | `number` | Frames between consecutive child entrances |
| `durationInFrames` | `number` | Scene length; used to clamp so no child enters past the scene's end |
| `children` | `ReactNode` | Each direct child enters independently |

Not for: simultaneous reveals (use `Reveal`), or children with uneven weights
(compute `beatSpans` yourself and pass each span's `from` as separate `at` values).

## Trace

An SVG path draws itself, stroke by stroke.

```tsx
<Trace name="Path" at={at(CUE.draw, durationInFrames)} until={at(CUE.drawn, durationInFrames)} d={pathData} />
```

| Prop | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | `string` | — | Studio label |
| `at` | `number` | — | Frame at which the stroke begins |
| `until` | `number` | — | Frame at which the stroke is complete |
| `d` | `string` | — | SVG path data |
| `stroke` | `string` | `THEME.accent` | Stroke colour |
| `strokeWidth` | `number` | `3` | Stroke width in px |
| `style` | `CSSProperties` | — | Applied to the SVG wrapper |

Uses `pathLength={1}` so no DOM measurement is needed; the path length is
normalised to 1 internally. The component fills its containing block absolutely
(`position: absolute; inset: 0`).

Not for: raster images or complex clip animations. Not for paths that must also
fill — `Trace` is stroke-only.

## Focus

A portion of the frame zooms in, directing the viewer's eye to a region.

```tsx
<Focus name="Detail" at={at(CUE.detail, durationInFrames)} on={{ x: 960, y: 540, scale: 1.5 }}>
  {/* full scene content or the portion to zoom */}
</Focus>
```

| Prop | Type | Description |
| --- | --- | --- |
| `name` | `string` | Studio label |
| `at` | `number` | Frame at which the zoom begins |
| `on.x` | `number` | Horizontal pivot of the zoom (px in the frame coordinate space) |
| `on.y` | `number` | Vertical pivot of the zoom (px in the frame coordinate space) |
| `on.scale` | `number` | Target zoom factor (e.g. `1.5` = 50% larger) |
| `children` | `ReactNode` | Content scaled around the pivot |

The transition duration is `FOCUS_DURATION = 20` frames (exported from
`motion-math.ts`).

Not for: camera cuts or perspective shifts — Focus is a CSS scale transform, not a
3D move. Not for zooms that must pan back out (add a second `Focus` at `on.scale: 1`).

## Emphasis

A word or phrase becomes the visual subject.

```tsx
<Emphasis name="Term" at={at(CUE.term, durationInFrames)} kind="underline">
  distributed tracing
</Emphasis>
```

| Prop | Type | Description |
| --- | --- | --- |
| `name` | `string` | Studio label |
| `at` | `number` | Frame at which the emphasis activates |
| `kind` | `"underline" \| "tint" \| "weight"` | Visual treatment |
| `children` | `ReactNode` | The text or element to mark |

**Kinds:**

- `underline` — an accent-coloured line draws itself left-to-right under the text.
- `tint` — a light accent-coloured background tint appears, with italic style as the
  non-colour indicator (the frame still communicates to a viewer who cannot
  distinguish the two tints, or to a still printed in greyscale).
- `weight` — the text transitions to `font-weight: 700`.

Emphasis never carries meaning through colour alone. Every kind includes a
shape, style, or weight change alongside any tint.

Not for: whole-paragraph highlights. Not for interactive or hover states — `at` is
a frame, not a pointer event. Not for error states or warnings (use a `Pill`).

## Count

A number climbs from one value to another over a span of frames.

```tsx
<Count name="Total" at={at(CUE.count, durationInFrames)} until={at(CUE.counted, durationInFrames)} from={0} to={374} />
```

| Prop | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | `string` | — | Studio label |
| `at` | `number` | — | Frame at which counting begins |
| `until` | `number` | — | Frame at which counting ends |
| `from` | `number` | — | Starting value |
| `to` | `number` | — | Ending value |
| `format` | `(n: number) => string` | `n => String(Math.round(n))` | How to render the interpolated value |
| `style` | `CSSProperties` | — | Additional styles |

Uses `tabular-nums` (`fontVariantNumeric`) so digits never jitter horizontally as
they change. Renders `inline`.

Not for: animated progress bars or percentage rings (those need a dedicated shape
component). Not for values that should animate non-linearly — `countValue` is
linear interpolation; apply easing in the `format` function if needed.
