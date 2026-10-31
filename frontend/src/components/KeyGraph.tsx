import { useEffect, useMemo, useRef, useState } from 'react'
import Box from '@mui/material/Box'
import { useTheme } from '@mui/material/styles'
import {
  forceCenter, forceCollide, forceLink, forceManyBody, forceSimulation, forceX, forceY,
  type SimulationLinkDatum, type SimulationNodeDatum,
} from 'd3-force'
import type { SharedKey } from '../types'
import { usePrefs } from '../store/preferences'

// The key-spread graph: owner → repository → key.
//
// A key node sits between every repository it was found in, so a credential
// pasted across organisations pulls their clusters together and shows as a
// bridge — the thing this view exists to make visible. Layout is d3-force; the
// drawing is plain SVG in React, so the nodes take the theme like everything
// else and no canvas has to be kept in sync with state.
//
// With reduced motion the simulation is run to rest before the first paint and
// drawn once, instead of settling on screen.

type Kind = 'owner' | 'repo' | 'key'

interface Node extends SimulationNodeDatum {
  id:    string
  kind:  Kind
  label: string
  key?:  SharedKey
  size:  number
}

interface Link extends SimulationLinkDatum<Node> {
  kind:   'own' | 'leak'
  atHead: boolean
  open:   boolean
}

interface Props {
  keys:      SharedKey[]
  selected:  string | null
  onSelect:  (keyId: string | null) => void
  height?:   number
  label:     string
}

function build(keys: SharedKey[]): { nodes: Node[]; links: Link[] } {
  // An owner node earns its place only when it groups two or more of the
  // repositories drawn here. Across a random corpus nearly every owner has one
  // repository, and a square per repository is noise that hides the bridges.
  const reposOf = new Map<string, Set<string>>()
  for (const k of keys) for (const r of k.repositories) {
    if (!reposOf.has(r.owner)) reposOf.set(r.owner, new Set())
    reposOf.get(r.owner)!.add(r.id)
  }
  const nodes = new Map<string, Node>()
  const links: Link[] = []
  for (const k of keys) {
    const kid = `k:${k.key_id}`
    nodes.set(kid, { id: kid, kind: 'key', label: k.masked_preview, key: k, size: 5 + Math.min(10, k.repositories.length * 1.5) })
    for (const r of k.repositories) {
      const rid = `r:${r.id}`, oid = `o:${r.owner}`
      const grouped = reposOf.get(r.owner)!.size > 1
      if (grouped && !nodes.has(oid)) {
        nodes.set(oid, { id: oid, kind: 'owner', label: r.owner, size: 8 })
      }
      if (!nodes.has(rid)) {
        nodes.set(rid, { id: rid, kind: 'repo', label: `${r.owner}/${r.name}`, size: 4 })
        if (grouped) links.push({ source: oid, target: rid, kind: 'own', atHead: false, open: true })
      }
      links.push({ source: kid, target: rid, kind: 'leak', atHead: r.reachable_from_head, open: r.status === 'open' })
    }
  }
  return { nodes: [...nodes.values()], links }
}

export default function KeyGraph({ keys, selected, onSelect, height = 520, label }: Props) {
  const theme = useTheme()
  const ch = theme.chart
  const { motionOff } = usePrefs()
  const box = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(800)
  const [, setTick] = useState(0)
  const [hover, setHover] = useState<string | null>(null)

  useEffect(() => {
    const el = box.current
    // Absent under jsdom and in some embedded browsers; the default width stands.
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(([e]) => setWidth(Math.max(320, Math.round(e.contentRect.width))))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const graph = useMemo(() => build(keys), [keys])

  useEffect(() => {
    const sim = forceSimulation<Node>(graph.nodes)
      .force('link', forceLink<Node, Link>(graph.links).id(n => n.id)
        .distance(l => (l.kind === 'own' ? 26 : 46)).strength(l => (l.kind === 'own' ? 0.9 : 0.5)))
      .force('charge', forceManyBody<Node>().strength(n => (n.kind === 'owner' ? -160 : n.kind === 'key' ? -110 : -40)).distanceMax(260))
      .force('collide', forceCollide<Node>(n => n.size + 4))
      .force('center', forceCenter(width / 2, height / 2))
      .force('x', forceX<Node>(width / 2).strength(0.07))
      .force('y', forceY<Node>(height / 2).strength(0.1))
    if (motionOff) {
      sim.stop()
      for (let i = 0; i < 300; i++) sim.tick()
      setTick(t => t + 1)
    } else {
      sim.on('tick', () => setTick(t => t + 1))
    }
    return () => { sim.stop() }
  }, [graph, width, height, motionOff])

  // The neighbourhood of whatever is hovered or selected stays lit; the rest dims.
  const focus = hover ?? (selected ? `k:${selected}` : null)
  const lit = useMemo(() => {
    if (!focus) return null
    const s = new Set<string>([focus])
    for (const l of graph.links) {
      const a = (l.source as Node).id, b = (l.target as Node).id
      if (a === focus || b === focus) { s.add(a); s.add(b) }
    }
    // A lit repository lights its owner too, so the bridge reads end to end.
    for (const l of graph.links) {
      if (l.kind !== 'own') continue
      const a = (l.source as Node).id, b = (l.target as Node).id
      if (s.has(b)) s.add(a)
    }
    return s
  }, [focus, graph])

  const dim = (id: string) => (lit && !lit.has(id) ? 0.15 : 1)
  const pos = (n: Node) => ({ x: n.x ?? 0, y: n.y ?? 0 })

  // The view is fitted to the graph rather than the graph clamped to the view:
  // clamping piles every repelled node along the border, where labels collide.
  const pad = 28
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity
  for (const n of graph.nodes) {
    x0 = Math.min(x0, (n.x ?? 0) - n.size); x1 = Math.max(x1, (n.x ?? 0) + n.size)
    y0 = Math.min(y0, (n.y ?? 0) - n.size - 14); y1 = Math.max(y1, (n.y ?? 0) + n.size)
  }
  if (!isFinite(x0)) { x0 = 0; y0 = 0; x1 = width; y1 = height }
  // Never zoom in past 1:1 — a small graph stays small rather than ballooning.
  const vw = Math.max(x1 - x0 + pad * 2, width), vh = Math.max(y1 - y0 + pad * 2, height)
  const cx = (x0 + x1) / 2, cy = (y0 + y1) / 2
  const viewBox = `${cx - vw / 2} ${cy - vh / 2} ${vw} ${vh}`

  return (
    <Box ref={box} sx={{ width: '100%', position: 'relative' }}>
      <svg width={width} height={height} viewBox={viewBox} preserveAspectRatio="xMidYMid meet" role="img" aria-label={label} style={{ display: 'block' }}
        onClick={() => onSelect(null)}>
        {graph.links.map((l, i) => {
          const a = pos(l.source as Node), b = pos(l.target as Node)
          const on = !lit || (lit.has((l.source as Node).id) && lit.has((l.target as Node).id))
          const stroke = l.kind === 'own' ? theme.palette.divider
            : l.atHead ? ch.severity.critical : theme.palette.text.secondary
          return (
            <line key={i} x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke={stroke}
              strokeWidth={l.kind === 'leak' ? 1.4 : 1} strokeDasharray={l.kind === 'leak' && !l.open ? '3 3' : undefined}
              opacity={on ? (l.kind === 'own' ? 0.8 : 0.7) : 0.06} />
          )
        })}
        {graph.nodes.map(n => {
          const { x, y } = pos(n)
          const o = dim(n.id)
          const common = {
            opacity: o, style: { cursor: n.kind === 'key' ? 'pointer' : 'default', transition: motionOff ? undefined : 'opacity .15s' },
            onMouseEnter: () => setHover(n.id), onMouseLeave: () => setHover(null),
            onClick: (e: React.MouseEvent) => { e.stopPropagation(); if (n.key) onSelect(n.key.key_id) },
          }
          if (n.kind === 'key') {
            const c = ch.severity[n.key!.severity] ?? ch.severity.medium
            const s = n.size
            const on = selected === n.key!.key_id
            return (
              <g key={n.id} {...common}>
                <title>{`${n.key!.provider || 'unmapped'} · ${n.label} · ${n.key!.repositories.length}`}</title>
                <polygon points={`${x},${y - s} ${x + s},${y} ${x},${y + s} ${x - s},${y}`}
                  fill={c} stroke={on ? theme.palette.text.primary : theme.palette.background.paper} strokeWidth={on ? 2.5 : 1.5} />
              </g>
            )
          }
          if (n.kind === 'owner') {
            return (
              <g key={n.id} {...common}>
                <title>{n.label}</title>
                <rect x={x - n.size} y={y - n.size} width={n.size * 2} height={n.size * 2} rx={2}
                  fill={theme.palette.text.primary} />
                <text x={x} y={y - n.size - 5} textAnchor="middle" fontSize={11} fontWeight={700}
                  fill={theme.palette.text.primary} style={{ pointerEvents: 'none', fontFamily: '"IBM Plex Mono", monospace' }}>
                  {n.label}
                </text>
              </g>
            )
          }
          const named = hover === n.id || (lit?.has(n.id) && !!selected)
          return (
            <g key={n.id} {...common}>
              <title>{n.label}</title>
              {named && (
                <text x={x} y={y - n.size - 4} textAnchor="middle" fontSize={10.5}
                  fill={theme.palette.text.secondary} style={{ pointerEvents: 'none', fontFamily: '"IBM Plex Mono", monospace' }}>
                  {n.label}
                </text>
              )}
              <circle cx={x} cy={y} r={n.size} fill={theme.palette.background.paper} stroke={theme.palette.text.secondary} strokeWidth={1.5} />
            </g>
          )
        })}
      </svg>
    </Box>
  )
}
