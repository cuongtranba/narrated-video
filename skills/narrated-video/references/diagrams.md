# Diagram scenes

How to render node/edge graphs in a `flow` scene, and why the `<Diagram>` wrapper
exists rather than importing `@xyflow/react` directly.

## The three silent failures

React Flow is built for an interactive canvas in a live browser. Remotion is the
opposite: no user, no clock, frames captured in parallel tabs that may each see only
one frame of the composition's life. Three of React Flow's defaults fail silently in
this environment — the frame renders, the process exits 0, the output is wrong.

**1. Node dimensions are measured, not declared.**

React Flow measures nodes with a `ResizeObserver` after first paint. A headless
renderer never fires that observer. Without explicit `width` and `height` on each
node, every node renders as a 0×0 point and every edge converges on that point.
Setting `width` and `height` directly on the node object bypasses measurement.
CHK-32 enforces this on `src/generated/diagrams.ts`.

**2. The viewport animates on a wall clock.**

`fitView` and any `setViewport` with a `duration` tween in real time. Two renders of
the same frame, started at different moments, see different viewports. `<Diagram>`
disables `fitView` and requires an explicit static `defaultViewport`.

**3. Interaction handlers keep state React Flow owns.**

Drag, pan, zoom, selection and connection all mutate an internal store. Anything the
composition does not derive from `useCurrentFrame()` can differ between two renders.
`<Diagram>` disables every interaction prop.

## The Diagram component

```tsx
import { Diagram, type DiagramGraph } from "kit/src/components/diagram"

const graph: DiagramGraph = {
  nodes: [
    { id: "input",   position: { x: 0,   y: 0 }, width: 220, height: 72, data: { label: "Input"   } },
    { id: "process", position: { x: 300, y: 0 }, width: 220, height: 72, data: { label: "Process" } },
    { id: "output",  position: { x: 600, y: 0 }, width: 220, height: 72, data: { label: "Output"  } },
  ],
  edges: [
    { id: "e1", source: "input",   target: "process" },
    { id: "e2", source: "process", target: "output"  },
  ],
}

<Diagram
  name="Pipeline"
  graph={graph}
  viewport={{ x: 410, y: 274, zoom: 1 }}
/>
```

### Props

| Prop | Type | Purpose |
|------|------|---------|
| `name` | `string` | Name passed to the Remotion interactive element inspector |
| `graph` | `DiagramGraph` | Nodes and edges; nodes must declare `width` and `height` |
| `viewport` | `{ x, y, zoom }` | Static viewport; never auto-derived (no `fitView`) |
| `reveal` | `{ at, through }` | Optional. Animates the walk — nodes `Reveal` in and stay, edges `Trace` in, in walk order. Omit for the static render above. |
| `subject` | `string` | Optional. A node id to hold as the visual subject once the walk has started — every other node dims to `0.35` opacity. |
| `flow` | `boolean \| { cycleFrames?, packets? }` | Optional. Runs packets along each edge once it has finished drawing. `cycleFrames` (default 45) is how long one packet takes to cross an edge; `packets` (default 2) is how many are in flight. |

## Flowing edges

A traced edge says two things are connected. A flowing edge says the connection
**carries**, which way, and how continuously — the difference between a topology
and a dataflow, and usually the thing the narration is actually about.

```tsx
<Diagram
  name="RequestPath"
  graph={graph}
  viewport={{ x: 150, y: 148, zoom: 1 }}
  reveal={{ at: at(CUE.walk, durationInFrames), through: at(CUE.walked, durationInFrames) }}
  flow={{ cycleFrames: 34, packets: 2 }}
/>
```

Flow begins where each edge's stroke completes, so a packet never appears on a
line the viewer has not watched being drawn. With no `reveal`, flow starts at
frame 0 and the graph is static-but-carrying.

Set an edge's `flowing: false` for a connection that exists but does not carry —
a fallback path, a cache miss that rarely happens, a control link among data
links. This is what makes the moving edges mean something: if everything flows,
flow is just decoration.

```ts
edges: data.edges.map(edge => ({ ...edge, flowing: edge.id !== "router-origin" })),
```

Under the hood this is `<Flow>` from `components/motion`, which animates
`stroke-dashoffset` against `pathLength={1}`. Nothing is measured from the DOM,
so the animation is identical in every capture tab — the same reason `Trace`
works headless. `flowDash` in `motion-math.ts` is the pure function, and it is
unit-tested for continuity and for staying inside one dash period.

Slower and fewer reads as deliberate traffic; faster and more reads as load. Two
packets at 34 frames is a good default for a four-hop path at 30fps.

## Walk order

`DiagramGraph.order` is the sequence `reveal` walks the graph in — the order nodes
enter and the order edges are anchored to. It is a plain `string[]` of node ids, and
it is optional: when absent, `Diagram` falls back to declaration order
(`graph.nodes.map(n => n.id)`).

For a graph produced by hand in a scene, declaration order is usually already the
right walk — write the nodes in the order the narration visits them. `order` exists
for graphs where the two diverge, or where a generator derives the walk itself: for
a DAG, the topological order (Kahn's algorithm, ties broken by declaration order so
the result is deterministic); for a graph containing a cycle, declaration order,
because there is no topological order to fall back to. `internal/gen/walk_order.go`
is the Go implementation of that derivation, used wherever `diagrams.ts` is
generated rather than hand-written.

## Never translate a node

`<Diagram>` fades nodes in with `rise={0}` — no movement — and that is a
correctness rule, not a taste one.

React Flow measures handle positions from the DOM and **caches** them. A node
that slides into place is, at the moment it is measured, displaced by `rise`.
The cached anchor keeps that displacement for the rest of the scene, so every
edge hangs below its node by `rise x zoom`.

What made it survive several rounds of fixing is that **it never appears in a
still**. `remotion still` renders one settled frame and measures the true
position. `remotion render` walks frames from zero, measures during the
entrance, and caches the wrong anchor for everything after. So the stills were
right and the video was wrong, and every check passed.

The lesson generalises past this component: for anything whose layout is
*measured* rather than declared, verify a frame extracted from the rendered
mp4, not a still.

## The walk schedule

`buildWalkSchedule` in `kit/src/motion-math.ts` divides `through - at` into one
step per node plus one per edge, and hands out two different kinds of cue:

- **A node gets an entrance frame and no exit.** It enters on its step and stays
  for the rest of the scene. The walk accumulates, which is the whole point — and
  it is what makes `subject` meaningful, since dimming every node but one requires
  the others to still be there.
- **An edge gets a span**, `{at, until}`, across which `Trace` draws its stroke.
  `until` here is the stroke completing, not the edge leaving.

The distinction is load-bearing because the two components read `until`
oppositely: on `Reveal` it is an exit (`opacity` returns to 0 over `until → until
+ 10`), on `Trace` it is completion. Feeding a node's `Reveal` the end of its walk
step faded every box out one step after it arrived, so a four-node build-up
rendered as an empty panel with stranded strokes — at exit 0, with all 42 checks
passing, because the gate reads no pixels. `motion-math.test.ts` now pins both
shapes.

An edge is anchored to `max(sourceSlot + 1, targetSlot)` — it traces only once
**both** boxes it joins are on screen. Anchoring to the source alone is correct on
a chain and wrong the moment the graph forks or joins: on the kit's own
`config + content → sync → generated` graph, the `config → sync` stroke reached
into empty canvas a full step before `sync` arrived.

### What is fixed inside the component and cannot be overridden by a caller

| Setting | Value | Why |
|---------|-------|-----|
| `fitView` | `false` | Would animate on a wall clock, producing a different viewport per render |
| `defaultViewport` | caller's value | Explicit and static; `fitView` would override it |
| `nodesDraggable` | `false` | Interaction state is not frame-derived |
| `nodesConnectable` | `false` | Same |
| `nodesFocusable` | `false` | Same |
| `edgesFocusable` | `false` | Same |
| `elementsSelectable` | `false` | Same |
| `panOnDrag` | `false` | Would produce a different viewport per render |
| `panOnScroll` | `false` | Same |
| `zoomOnScroll` | `false` | Same |
| `zoomOnPinch` | `false` | Same |
| `zoomOnDoubleClick` | `false` | Same |
| `preventScrolling` | `false` | Headless renderer has no scroll to prevent |

## CSS import

`@xyflow/react/dist/base.css` is imported inside `diagram.tsx` (not `style.css`).
`base.css` contains the layout rules the library needs — the edge SVG positioning,
the handle placement — and none of its default node theming (borders, font sizes,
white backgrounds). `style.css` would override the kit's own node styles.

## Authoring loop

Diagram data splits across two files:

- `video.config.yaml` owns **topology** — nodes (id, position, size) and edges. The Go
  validator reads this without executing TypeScript.
- `content/<locale>.yaml` owns **labels** — the human-readable text for each node, one
  entry per locale. A translator opens this file.

`nv sync` derives `src/generated/diagrams.ts` from both, producing one typed object per
diagram per locale. The generated file is committed, like `timeline.ts`, and CHK-01 catches
any drift between what is on disk and what a fresh derivation would produce.

### Config block

```yaml
# video.config.yaml
diagrams:
  pipeline:
    nodes:
      - id: config
        at: [0, 0]
        size: [320, 96]
      - id: sync
        at: [420, 0]
        size: [320, 96]
    edges:
      - from: config
        to: sync
```

`at` is the node's `[x, y]` position in React Flow canvas coordinates. `size` is
`[width, height]`. Both are required — `nv sync` uses them to populate the fields that
CHK-32 requires on every node.

### Content block

```yaml
# content/en.yaml
copy:
  diagrams:
    pipeline:
      config: "video.config.yaml"
      sync: "nv sync"
```

Every locale must have an entry for every node id. CHK-30 catches a missing or empty
label before the render.

### Generated file

After `nv sync`, `src/generated/diagrams.ts` contains:

```typescript
export const DIAGRAMS: Readonly<Record<Locale, Readonly<Record<string, DiagramData>>>> = {
  en: {
    pipeline: {
      nodes: [
        { id: "config", x: 0, y: 0, width: 320, height: 96, label: "video.config.yaml" },
        { id: "sync",   x: 420, y: 0, width: 320, height: 96, label: "nv sync" },
      ],
      edges: [
        { id: "config-sync", source: "config", target: "sync" },
      ],
    },
  },
  // … other locales …
}
```

Import `DIAGRAMS` in a scene alongside `TIMELINE` to build the `DiagramGraph` for the
current locale at the current frame.

## Checks

**CHK-29** — every diagram edge references nodes declared in the same diagram.

Reads `video.config.yaml` and checks each edge's `from` and `to` against the node ids in
the same diagram. A typo'd endpoint renders an edge to nowhere while the process exits 0.

**CHK-30** — every diagram node has a label in every locale.

Reads `video.config.yaml` and every `content/<locale>.yaml`. Fails when a node has no
label, or an empty label, in any locale. Catches localization gaps before the render
produces a frame with a bare key or an empty box.

**CHK-32** — every diagram node declares `width` and `height`.

Runs against `src/generated/diagrams.ts`. Passes vacuously when that file does not
exist. A node without both fields renders as 0×0 in a headless renderer while the
process exits 0.

**CHK-36** — scene modules import React Flow only through `<Diagram>`.

Scans scene sources for direct `@xyflow/react` imports. The wrapper is where all
the determinism guards live; bypassing it loses them silently.

## Initialising a flow scene

```bash
nv init --scene Pipeline --kind flow
```

This copies `_template.flow.tsx` into `src/scenes/Pipeline.tsx` and adds
`@xyflow/react` to `package.json`. The template renders a three-node pipeline
graph and passes the gate without modification.

## Positioning nodes

Nodes are positioned in React Flow's canvas coordinate system. The origin is the
top-left of the canvas at zoom 1. To centre a graph in its container, compute:

```
viewport.x = (containerWidth  - graphWidth)  / 2
viewport.y = (containerHeight - graphHeight) / 2
```

where `graphWidth = maxNodeX + nodeWidth` and `graphHeight = maxNodeY + nodeHeight`.
