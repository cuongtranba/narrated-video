import "@xyflow/react/dist/base.css"

import { Handle, Position, ReactFlow, type NodeProps } from "@xyflow/react"
import React from "react"

import { HAIRLINE, RADIUS, SIZE } from "./primitives"
import { THEME } from "../generated/theme"

export type DiagramNode = {
  id: string
  position: { x: number; y: number }
  width: number
  height: number
  data: { label: string }
}

export type DiagramEdge = {
  id: string
  source: string
  target: string
  sourceHandle?: string
  targetHandle?: string
}

export type DiagramGraph = {
  nodes: DiagramNode[]
  edges: DiagramEdge[]
}

const KitNode: React.FC<NodeProps> = ({ data, width, height }) => (
  <div
    style={{
      width,
      height,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      border: HAIRLINE,
      borderRadius: RADIUS.md,
      backgroundColor: THEME.surface,
      color: THEME.foreground,
      fontSize: SIZE.mono,
      fontWeight: 500,
      letterSpacing: "-0.01em",
    }}
  >
    <Handle type="target" position={Position.Left} style={{ opacity: 0, pointerEvents: "none" }} />
    {String(data.label)}
    <Handle type="source" position={Position.Right} style={{ opacity: 0, pointerEvents: "none" }} />
  </div>
)

const nodeTypes = { kit: KitNode }

export const Diagram: React.FC<{
  name: string
  graph: DiagramGraph
  viewport: { x: number; y: number; zoom: number }
}> = ({ name, graph, viewport }) => {
  const nodes = graph.nodes.map(n => ({ ...n, type: "kit" as const }))

  return (
    <div
      data-remotion-interactive={name}
      style={{ width: "100%", height: "100%", backgroundColor: THEME.surface, borderRadius: RADIUS.lg }}
    >
      <ReactFlow
        nodes={nodes}
        edges={graph.edges}
        nodeTypes={nodeTypes}
        defaultViewport={viewport}
        fitView={false}
        nodesDraggable={false}
        nodesConnectable={false}
        nodesFocusable={false}
        edgesFocusable={false}
        elementsSelectable={false}
        panOnDrag={false}
        panOnScroll={false}
        zoomOnScroll={false}
        zoomOnPinch={false}
        zoomOnDoubleClick={false}
        preventScrolling={false}
        proOptions={{ hideAttribution: true }}
      />
    </div>
  )
}
