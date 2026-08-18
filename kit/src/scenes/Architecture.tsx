import { Diagram, type DiagramGraph } from "../components/diagram"
import { Reveal, Rule } from "../components/primitives"
import { Stage } from "../components/stage"
import { THEME } from "../generated/theme"
import { at } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { diagram: 0.2 }

const graph: DiagramGraph = {
  nodes: [
    { id: "config", position: { x: 0, y: 0 }, width: 280, height: 72, data: { label: "video.config.yaml" } },
    { id: "content", position: { x: 0, y: 120 }, width: 280, height: 72, data: { label: "content/*.yaml" } },
    { id: "sync", position: { x: 360, y: 60 }, width: 180, height: 72, data: { label: "nv sync" } },
    { id: "generated", position: { x: 620, y: 60 }, width: 280, height: 72, data: { label: "src/generated/" } },
  ],
  edges: [
    { id: "e1", source: "config", target: "sync" },
    { id: "e2", source: "content", target: "sync" },
    { id: "e3", source: "sync", target: "generated" },
  ],
}

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => (
  <Stage name="Architecture" heading="The pipeline">
    <div
      style={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        gap: 36,
      }}
    >
      <Rule at={leadFrames} width={480} color={THEME.accent} />

      <Reveal name="Diagram" at={at(CUE.diagram, durationInFrames)}>
        <div style={{ height: 520 }}>
          <Diagram
            name="Architecture"
            graph={graph}
            viewport={{ x: 350, y: 210, zoom: 1 }}
          />
        </div>
      </Reveal>
    </div>
  </Stage>
)
