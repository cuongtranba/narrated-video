export const FOCUS_DURATION = 20

export interface FocusOn {
  readonly x: number
  readonly y: number
  readonly scale: number
}

export interface FocusTransformResult {
  readonly x: number
  readonly y: number
  readonly scale: number
}

export function staggerOffset(
  childIndex: number,
  step: number,
  childCount: number,
  sceneDuration: number,
): number {
  const maxOffset = Math.max(0, sceneDuration - 1)
  const totalSpan = (childCount - 1) * step
  if (totalSpan <= maxOffset) {
    return childIndex * step
  }
  const compressedStep = childCount > 1 ? Math.floor(maxOffset / (childCount - 1)) : 0
  return Math.min(childIndex * compressedStep, maxOffset)
}

export function countValue(
  frame: number,
  atFrame: number,
  untilFrame: number,
  from: number,
  to: number,
): number {
  if (frame <= atFrame) return from
  if (frame >= untilFrame) return to
  const progress = (frame - atFrame) / (untilFrame - atFrame)
  return from + progress * (to - from)
}

export function focusTransform(
  frame: number,
  atFrame: number,
  on: FocusOn,
): FocusTransformResult {
  const progress = Math.min(Math.max((frame - atFrame) / FOCUS_DURATION, 0), 1)
  return {
    x: on.x * progress,
    y: on.y * progress,
    scale: 1 + (on.scale - 1) * progress,
  }
}

/**
 * When one item of a diagram walk enters, and — for an edge only — when it has
 * finished drawing.
 *
 * `until` is optional because the two consumers read it differently, and the
 * walk must not hand the same window to both. `Trace` treats `until` as the
 * frame the stroke FINISHES; an edge needs one. `Reveal` treats `until` as the
 * frame the element FADES OUT; a node must not have one.
 */
export interface WalkWindow {
  readonly at: number
  readonly until?: number
}

export interface WalkSchedule {
  readonly nodes: ReadonlyMap<string, WalkWindow>
  readonly edges: ReadonlyMap<string, WalkWindow>
}

export interface WalkEdge {
  readonly id: string
  readonly source: string
}

/**
 * Spread a graph's nodes and edges across `at`..`through` so the diagram reads
 * as a walk: a node enters, then the edge leaving it draws, then the next node.
 *
 * An edge is anchored to its SOURCE node's position in the walk rather than to
 * its own index, which is what stops the viewer ever seeing an arrow point at a
 * box that has not arrived yet.
 *
 * Nodes are scheduled with no end frame ON PURPOSE — see `WalkWindow`. They
 * once carried `until: at + step`, which fed `Reveal` a fade-out and left the
 * finished diagram as a row of edges pointing at nothing.
 */
export function walkSchedule(
  nodeIds: readonly string[],
  edges: readonly WalkEdge[],
  at: number,
  through: number,
): WalkSchedule {
  const step = Math.max(1, Math.floor((through - at) / (nodeIds.length + edges.length + 1)))

  const nodes = new Map<string, WalkWindow>()
  nodeIds.forEach((id, i) => {
    nodes.set(id, { at: at + i * step })
  })

  const edgeWindows = new Map<string, WalkWindow>()
  for (const edge of edges) {
    const enters = at + (nodeIds.indexOf(edge.source) + 1) * step
    edgeWindows.set(edge.id, { at: enters, until: enters + step })
  }

  return { nodes, edges: edgeWindows }
}
