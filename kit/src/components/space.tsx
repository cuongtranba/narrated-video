import { ThreeCanvas } from "@remotion/three"
import React from "react"
import { useVideoConfig } from "remotion"

import { THEME } from "../generated/theme"

type Vec3 = [number, number, number]

export type SpaceCamera = {
  position: Vec3
  fov?: number
}

export const Space: React.FC<{
  name: string
  camera?: SpaceCamera
  children?: React.ReactNode
}> = ({ name, camera = { position: [0, 0, 5], fov: 75 }, children }) => {
  const { width, height } = useVideoConfig()

  return (
    <div
      data-remotion-interactive={name}
      style={{ width: "100%", height: "100%" }}
    >
      <ThreeCanvas
        width={width}
        height={height}
        style={{ backgroundColor: "transparent" }}
        camera={{ fov: camera.fov ?? 75, position: camera.position }}
      >
        <ambientLight color={THEME.foreground} intensity={0.15} />
        <directionalLight color={THEME.foreground} position={[5, 5, 5]} intensity={0.8} />
        {children}
      </ThreeCanvas>
    </div>
  )
}
