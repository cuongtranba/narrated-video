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
