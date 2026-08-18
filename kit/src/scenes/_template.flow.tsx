import { Diagram, type DiagramGraph } from "../components/diagram"
import { Reveal, Rule } from "../components/primitives"
import { Stage } from "../components/stage"
import { THEME } from "../generated/theme"
import { at } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { diagram: 0.2 }

const graph: DiagramGraph = {
  nodes: [
    { id: "input", position: { x: 0, y: 0 }, width: 220, height: 72, data: { label: "Input" } },
    { id: "process", position: { x: 300, y: 0 }, width: 220, height: 72, data: { label: "Process" } },
    { id: "output", position: { x: 600, y: 0 }, width: 220, height: 72, data: { label: "Output" } },
  ],
  edges: [
    { id: "e1", source: "input", target: "process" },
    { id: "e2", source: "process", target: "output" },
  ],
}

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
        <div style={{ height: 620 }}>
          <Diagram
            name="Pipeline"
            graph={graph}
            viewport={{ x: 410, y: 274, zoom: 1 }}
          />
        </div>
      </Reveal>
    </div>
  </Stage>
)
