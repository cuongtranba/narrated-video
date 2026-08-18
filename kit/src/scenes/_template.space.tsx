import { useCurrentFrame } from "remotion"

import { Space } from "../components/space"
import { Reveal, Rule } from "../components/primitives"
import { Stage } from "../components/stage"
import { THEME } from "../generated/theme"
import { at } from "../timing"
import type { SceneComponent } from "./types"

const CUE = { space: 0.2 }

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => {
  const frame = useCurrentFrame()
  const rotation = frame * 0.04

  return (
    <Stage name="SpaceTemplate" heading="Replace this heading">
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

        <Reveal name="Space" at={at(CUE.space, durationInFrames)}>
          <div style={{ height: 620 }}>
            <Space name="Cube" camera={{ position: [0, 0, 4], fov: 60 }}>
              <mesh rotation={[rotation * 0.7, rotation, 0]}>
                <boxGeometry args={[1.5, 1.5, 1.5]} />
                <meshStandardMaterial color={THEME.accent} />
              </mesh>
            </Space>
          </div>
        </Reveal>
      </div>
    </Stage>
  )
}
