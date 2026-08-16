import React from "react"
import { Easing, Interactive, interpolate, useCurrentFrame } from "remotion"

import { monoFamily, tabular } from "../fonts"
import { THEME } from "../generated/theme"

/**
 * The geometric scale, hand-written where `THEME` is generated.
 *
 * Colour is data the config owns and a translator or a designer can change
 * without opening TypeScript. A type scale and a safe area are layout
 * decisions these components make — and `theme` in video.config.yaml holds
 * strings, so putting `safeX: "140"` there would buy a `parseInt` and no
 * ownership.
 *
 * Sizes are an app's hierarchy multiplied for viewing distance, not a new one:
 * a 1920x1080 frame is read from a couch, not from a desk.
 */
export const SAFE = { x: 140, y: 110 } as const

export const SIZE = {
  display: 108,
  heading: 66,
  lead: 46,
  body: 34,
  mono: 30,
  label: 26,
} as const

export const RADIUS = { sm: 6, md: 10, lg: 16 } as const

/** Kanna-style ease-out. One curve, so nothing in the cut moves differently. */
export const EASE_OUT = [0.16, 1, 0.3, 1] as const

/** A 1px border at 1080p is invisible on a phone; 2px is what 1px looks like. */
export const HAIRLINE = `2px solid ${THEME.border}`

/**
 * The one entrance in this video: rise and fade, on the shared curve.
 *
 * Every element that appears goes through here, so timing is expressed at the
 * call site as two plain numbers (`at`, and optionally `until`) while the curve
 * stays in one place. Scenes then read as a score — a column of frame numbers —
 * instead of forty copies of the same interpolate.
 */
export const Reveal: React.FC<{
  name: string
  at: number
  until?: number
  rise?: number
  style?: React.CSSProperties
  children: React.ReactNode
}> = ({ name, at, until, rise = 26, style, children }) => {
  const frame = useCurrentFrame()
  const exit = until ?? Number.MAX_SAFE_INTEGER

  return (
    <Interactive.Div
      name={name}
      style={{
        ...style,
        opacity: interpolate(frame, [at, at + 12, exit, exit + 10], [0, 1, 1, 0], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing: Easing.bezier(...EASE_OUT),
        }),
        translate: interpolate(frame, [at, at + 16], [`0px ${rise}px`, "0px 0px"], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing: Easing.bezier(...EASE_OUT),
        }),
      }}
    >
      {children}
    </Interactive.Div>
  )
}

/** A flat surface. Depth is the 1px edge and nothing else — never a shadow. */
export const Card: React.FC<{
  style?: React.CSSProperties
  children: React.ReactNode
}> = ({ style, children }) => (
  <div
    style={{
      backgroundColor: THEME.surface,
      border: HAIRLINE,
      borderRadius: RADIUS.lg,
      padding: "26px 32px",
      ...style,
    }}
  >
    {children}
  </div>
)

/** Monospace for anything the system itself writes: calls, ids, exit codes. */
export const Mono: React.FC<{
  children: React.ReactNode
  color?: string
  fontSize?: number
  style?: React.CSSProperties
}> = ({ children, color = THEME.muted, fontSize = SIZE.mono, style }) => (
  <span style={{ fontFamily: monoFamily, fontSize, color, ...tabular, ...style }}>{children}</span>
)

/**
 * A status marker. Colour never travels alone — the pill always carries its own
 * label, so the frame still communicates to a viewer who cannot separate the
 * two tints, or to a still printed in grey.
 */
export const Pill: React.FC<{ label: string; tint: string; muted?: boolean }> = ({
  label,
  tint,
  muted = false,
}) => (
  <span
    style={{
      display: "inline-flex",
      alignItems: "center",
      gap: 12,
      fontWeight: 500,
      fontSize: SIZE.label,
      color: muted ? THEME.muted : THEME.foreground,
      border: `2px solid ${muted ? THEME.border : tint}`,
      borderRadius: 999,
      padding: "7px 20px",
      whiteSpace: "nowrap",
    }}
  >
    <span
      style={{
        width: 13,
        height: 13,
        borderRadius: 999,
        backgroundColor: muted ? THEME.border : tint,
      }}
    />
    {label}
  </span>
)

/** A hairline divider that draws itself in, used to pace dense scenes. */
export const Rule: React.FC<{ at: number; width: number; color?: string }> = ({
  at,
  width,
  color = THEME.border,
}) => {
  const frame = useCurrentFrame()

  return (
    <div
      style={{
        height: 2,
        backgroundColor: color,
        width: interpolate(frame, [at, at + 22], [0, width], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing: Easing.bezier(...EASE_OUT),
        }),
      }}
    />
  )
}
