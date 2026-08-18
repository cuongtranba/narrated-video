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
| `reveal` | `{ at, through }` | Optional. Animates the walk — nodes `Stagger` in, edges `Trace` in, in walk order. Omit for the static render above. |
| `subject` | `string` | Optional. A node id to hold as the visual subject once the walk has started — every other node dims to `0.35` opacity. |

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

## Edge ordering

An edge's reveal is anchored to its source node's position in the walk, not to its
own position in `graph.edges`: an edge traces in only after its source node has
entered, and finishes tracing before the walk reaches whatever comes after it. This
is what makes the animation read as a walk rather than a simultaneous unveiling —
the viewer never sees an edge pointing at a node that has not appeared yet.

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
