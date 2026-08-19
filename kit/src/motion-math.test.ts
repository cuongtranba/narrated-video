import { describe, expect, test } from "bun:test"

import {
  countValue,
  FOCUS_DURATION,
  focusTransform,
  staggerOffset,
  walkSchedule,
} from "./motion-math"

describe("staggerOffset", () => {
  test("child 0 has no offset — it enters at the at cue itself", () => {
    expect(staggerOffset(0, 4, 5, 100)).toBe(0)
  })

  test("children enter step frames apart", () => {
    expect(staggerOffset(1, 4, 5, 100)).toBe(4)
    expect(staggerOffset(2, 4, 5, 100)).toBe(8)
    expect(staggerOffset(3, 4, 5, 100)).toBe(12)
  })

  test("never schedules a child past the scene's end", () => {
    const step = 30
    const childCount = 5
    const sceneDuration = 60
    for (let i = 0; i < childCount; i++) {
      expect(staggerOffset(i, step, childCount, sceneDuration)).toBeLessThan(sceneDuration)
    }
  })

  test("determinism: same arguments always yield the same offset", () => {
    const result1 = staggerOffset(2, 4, 5, 100)
    const result2 = staggerOffset(2, 4, 5, 100)
    expect(result1).toBe(result2)
  })

  test("offsets are non-negative", () => {
    for (let i = 0; i < 5; i++) {
      expect(staggerOffset(i, 4, 5, 100)).toBeGreaterThanOrEqual(0)
    }
  })
})

describe("countValue", () => {
  test("before at: returns from", () => {
    expect(countValue(0, 10, 50, 0, 374)).toBe(0)
    expect(countValue(9, 10, 50, 0, 374)).toBe(0)
  })

  test("at or past until: returns to", () => {
    expect(countValue(50, 10, 50, 0, 374)).toBe(374)
    expect(countValue(60, 10, 50, 0, 374)).toBe(374)
  })

  test("between at and until: interpolates strictly between from and to", () => {
    const mid = countValue(30, 10, 50, 0, 374)
    expect(mid).toBeGreaterThan(0)
    expect(mid).toBeLessThan(374)
  })

  test("determinism: same frame always yields the same value", () => {
    const result1 = countValue(30, 10, 50, 0, 374)
    const result2 = countValue(30, 10, 50, 0, 374)
    expect(result1).toBe(result2)
  })

  test("interpolation is monotonically non-decreasing when from < to", () => {
    const vals = [10, 20, 30, 40, 50].map((f) => countValue(f, 10, 50, 0, 374))
    for (let i = 1; i < vals.length; i++) {
      expect(vals[i]).toBeGreaterThanOrEqual(vals[i - 1]!)
    }
  })
})

describe("focusTransform", () => {
  const on = { x: 500, y: 300, scale: 2 }

  test("before at: identity transform — no zoom, no translate", () => {
    const result = focusTransform(0, 10, on)
    expect(result.x).toBe(0)
    expect(result.y).toBe(0)
    expect(result.scale).toBe(1)
  })

  test("at + FOCUS_DURATION: full transform applied", () => {
    const result = focusTransform(10 + FOCUS_DURATION, 10, on)
    expect(result.x).toBe(on.x)
    expect(result.y).toBe(on.y)
    expect(result.scale).toBe(on.scale)
  })

  test("determinism: same frame always yields the same transform", () => {
    const r1 = focusTransform(15, 10, on)
    const r2 = focusTransform(15, 10, on)
    expect(r1).toEqual(r2)
  })

  test("scale stays at or above 1 during transition", () => {
    for (let f = 0; f <= 10 + FOCUS_DURATION; f++) {
      expect(focusTransform(f, 10, on).scale).toBeGreaterThanOrEqual(1)
    }
  })
})

describe("walkSchedule", () => {
  const NODES = ["a", "b", "c"]
  const EDGES = [
    { id: "a-b", source: "a" },
    { id: "b-c", source: "b" },
  ]

  test("nodes enter in walk order, one step apart, starting at the at cue", () => {
    const { nodes } = walkSchedule(NODES, EDGES, 0, 60)
    // 5 items + 1 => step 10
    expect(nodes.get("a")?.at).toBe(0)
    expect(nodes.get("b")?.at).toBe(10)
    expect(nodes.get("c")?.at).toBe(20)
  })

  test("a node NEVER carries an until — Reveal reads until as a fade-out", () => {
    // The regression this covers: nodes were scheduled `until: at + step`, so
    // every box faded out one step after arriving and the finished diagram was
    // a row of edges pointing at nothing. `subject` (hold one node, dim the
    // rest) is only meaningful if nodes persist.
    const { nodes } = walkSchedule(NODES, EDGES, 0, 60)
    for (const id of NODES) {
      expect(nodes.get(id)?.until).toBeUndefined()
    }
  })

  test("an edge carries a finite until — Trace reads until as stroke-complete", () => {
    const { edges } = walkSchedule(NODES, EDGES, 0, 60)
    for (const edge of EDGES) {
      const window = edges.get(edge.id)
      expect(window?.until).toBeGreaterThan(window?.at ?? 0)
    }
  })

  test("an edge is anchored after its source node has entered", () => {
    const { nodes, edges } = walkSchedule(NODES, EDGES, 0, 60)
    for (const edge of EDGES) {
      expect(edges.get(edge.id)?.at).toBeGreaterThan(nodes.get(edge.source)?.at ?? 0)
    }
  })

  test("step is at least 1 even when the window is shorter than the item count", () => {
    const { nodes } = walkSchedule(NODES, EDGES, 0, 1)
    expect(nodes.get("b")?.at).toBe(1)
    expect(nodes.get("c")?.at).toBe(2)
  })

  test("determinism: same arguments always yield the same schedule", () => {
    const first = walkSchedule(NODES, EDGES, 5, 90)
    const second = walkSchedule(NODES, EDGES, 5, 90)
    expect([...first.nodes]).toEqual([...second.nodes])
    expect([...first.edges]).toEqual([...second.edges])
  })
})
