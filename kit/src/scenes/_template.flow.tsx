import { HAIRLINE, RADIUS, Reveal, Rule } from "../components/primitives"
import { Stage } from "../components/stage"
import { THEME } from "../generated/theme"
import { at } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { diagram: 0.2 }

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => (
  <Stage name="FlowTemplate" heading="Replace this heading">
    <div
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        gap: 36,
      }}
    >
      <Rule at={leadFrames} width={520} color={THEME.accent} />

      <Reveal name="Diagram" at={at(CUE.diagram, durationInFrames)}>
        <div
          style={{
            height: 620,
            border: HAIRLINE,
            borderRadius: RADIUS.lg,
            backgroundColor: THEME.surface,
          }}
        />
      </Reveal>
    </div>
  </Stage>
)
