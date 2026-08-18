import "@xyflow/react/dist/base.css"

import {
  EdgeLabelRenderer,
  getBezierPath,
  Handle,
  Position,
  ReactFlow,
  type EdgeProps,
  type NodeProps,
} from "@xyflow/react"
import React from "react"
import { useCurrentFrame } from "remotion"

import { Trace } from "./motion"
import { HAIRLINE, RADIUS, Reveal, SIZE } from "./primitives"
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
  order?: string[]
}

export type DiagramReveal = {
  at: number
  through: number
}

const MUTED_NODE_OPACITY = 0.35

type RevealWindow = { at: number; until: number }

type WalkSchedule = {
  nodeReveal: Map<string, RevealWindow>
  edgeReveal: Map<string, RevealWindow>
}

const buildWalkSchedule = (graph: DiagramGraph, reveal: DiagramReveal): WalkSchedule => {
  const walkOrder = graph.order ?? graph.nodes.map(node => node.id)
  const totalItems = graph.nodes.length + graph.edges.length
  const step = Math.max(1, Math.floor((reveal.through - reveal.at) / (totalItems + 1)))

  const nodeReveal = new Map<string, RevealWindow>()
  walkOrder.forEach((id, i) => {
    const at = reveal.at + i * step
    nodeReveal.set(id, { at, until: at + step })
  })

  const edgeReveal = new Map<string, RevealWindow>()
  for (const edge of graph.edges) {
    const at = reveal.at + (walkOrder.indexOf(edge.source) + 1) * step
    edgeReveal.set(edge.id, { at, until: at + step })
  }

  return { nodeReveal, edgeReveal }
}

type KitNodeData = {
  label: string
  revealAt?: number
  revealUntil?: number
  hasSubject?: boolean
  isSubject?: boolean
}

const KitNode: React.FC<NodeProps> = ({ id, data, width, height }) => {
  const nodeData = data as KitNodeData
  const frame = useCurrentFrame()
  const muted =
    Boolean(nodeData.hasSubject) &&
    !nodeData.isSubject &&
    nodeData.revealAt !== undefined &&
    frame >= nodeData.revealAt

  const body = (
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
        opacity: muted ? MUTED_NODE_OPACITY : 1,
      }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0, pointerEvents: "none" }} />
      {nodeData.label}
      <Handle type="source" position={Position.Right} style={{ opacity: 0, pointerEvents: "none" }} />
    </div>
  )

  if (nodeData.revealAt === undefined) {
    return body
  }

  return (
    <Reveal name={`node-${id}`} at={nodeData.revealAt} until={nodeData.revealUntil}>
      {body}
    </Reveal>
  )
}

const nodeTypes = { kit: KitNode }

type KitEdgeData = {
  revealAt?: number
  revealUntil?: number
}

const KitEdge: React.FC<EdgeProps> = ({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}) => {
  const edgeData = data as KitEdgeData | undefined
  if (!edgeData || edgeData.revealAt === undefined || edgeData.revealUntil === undefined) {
    return null
  }

  const [d] = getBezierPath({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition })

  return (
    <EdgeLabelRenderer>
      <Trace name={`edge-${id}`} at={edgeData.revealAt} until={edgeData.revealUntil} d={d} />
    </EdgeLabelRenderer>
  )
}

const edgeTypes = { kit: KitEdge }

export const Diagram: React.FC<{
  name: string
  graph: DiagramGraph
  viewport: { x: number; y: number; zoom: number }
  reveal?: DiagramReveal
  subject?: string
}> = ({ name, graph, viewport, reveal, subject }) => {
  const schedule = reveal ? buildWalkSchedule(graph, reveal) : undefined

  const nodes = graph.nodes.map(node =>
    schedule
      ? {
          ...node,
          type: "kit" as const,
          data: {
            label: node.data.label,
            revealAt: schedule.nodeReveal.get(node.id)?.at,
            revealUntil: schedule.nodeReveal.get(node.id)?.until,
            hasSubject: subject !== undefined,
            isSubject: node.id === subject,
          },
        }
      : { ...node, type: "kit" as const },
  )

  const edges = schedule
    ? graph.edges.map(edge => ({
        ...edge,
        type: "kit" as const,
        data: {
          revealAt: schedule.edgeReveal.get(edge.id)?.at,
          revealUntil: schedule.edgeReveal.get(edge.id)?.until,
        },
      }))
    : graph.edges

  return (
    <div
      data-remotion-interactive={name}
      style={{ width: "100%", height: "100%", backgroundColor: THEME.surface, borderRadius: RADIUS.lg }}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
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
