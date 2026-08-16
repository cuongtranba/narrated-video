import { Reveal, Rule, SIZE } from "../components/primitives"
import { Stage } from "../components/stage"
import { useCopy } from "../content/use-copy"
import { THEME } from "../generated/theme"
import { at } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { subheading: 0.34 }

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => {
  const { title } = useCopy()

  return (
    <Stage name="Title">
      <div
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          gap: 34,
        }}
      >
        {/* At 0, so the title has settled by the time the first word lands. */}
        <Reveal name="Heading" at={0}>
          <h1
            style={{
              margin: 0,
              fontWeight: 700,
              fontSize: SIZE.display,
              letterSpacing: "-0.02em",
              lineHeight: 1.05,
              maxWidth: 1500,
            }}
          >
            {title.heading}
          </h1>
        </Reveal>

        {/* Draws itself in on the first word — the lead is exactly that cue. */}
        <Rule at={leadFrames} width={420} color={THEME.accent} />

        <Reveal name="Subheading" at={at(CUE.subheading, durationInFrames)}>
          <p
            style={{
              margin: 0,
              fontSize: SIZE.lead,
              lineHeight: 1.4,
              color: THEME.muted,
              maxWidth: 1400,
            }}
          >
            {title.subheading}
          </p>
        </Reveal>
      </div>
    </Stage>
  )
}
