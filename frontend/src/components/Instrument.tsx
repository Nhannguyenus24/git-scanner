import { useEffect, useRef, useState } from 'react'
import { keyframes } from '@emotion/react'
import Box from '@mui/material/Box'
import LinearProgress, { type LinearProgressProps } from '@mui/material/LinearProgress'
import { usePrefs } from '../store/preferences'

// Small pieces of instrument hardware: registration ticks, a live indicator, a
// counter that rolls to its reading, a scan bar with a sweep. They add motion and
// detail without changing the stock — hairlines, mono type, theme ink, no drop
// shadows — so every preset keeps its own character.
//
// Every animation here is CSS, so the theme's reduced-motion rule (OS or
// appearance setting) cuts it to one frame. The one JS animation, CountUp, asks
// the preference itself.

const pulse = keyframes`
  0%   { transform: scale(1);   opacity: .55 }
  70%  { transform: scale(2.6); opacity: 0 }
  100% { transform: scale(2.6); opacity: 0 }
`

const sweep = keyframes`
  0%   { transform: translateX(-100%) }
  100% { transform: translateX(400%) }
`

const spin = keyframes`
  to { transform: rotate(360deg) }
`

const blink = keyframes`
  50% { opacity: 0 }
`

/**
 * Registration marks at the four corners of a positioned parent, like the
 * crop ticks on a plotter sheet. Purely decorative; the parent needs
 * `position: relative`.
 */
export function CornerTicks({ color = 'divider', size = 7, inset = 4 }: { color?: string; size?: number; inset?: number }) {
  const tick = (v: 'top' | 'bottom', h: 'left' | 'right') => (
    <Box key={v + h} sx={{
      position: 'absolute', [v]: inset, [h]: inset, width: size, height: size,
      [`border${v === 'top' ? 'Top' : 'Bottom'}`]: 1, [`border${h === 'left' ? 'Left' : 'Right'}`]: 1,
      borderColor: color, pointerEvents: 'none', transition: 'border-color .15s',
    }} />
  )
  return <Box aria-hidden sx={{ position: 'absolute', inset: 0, pointerEvents: 'none' }} className="corner-ticks">
    {tick('top', 'left')}{tick('top', 'right')}{tick('bottom', 'left')}{tick('bottom', 'right')}
  </Box>
}

/** A status dot that breathes while something is live. */
export function LiveDot({ color = 'success.main', live = true, size = 8 }: { color?: string; live?: boolean; size?: number }) {
  return (
    <Box component="span" aria-hidden sx={{ position: 'relative', display: 'inline-block', width: size, height: size, flexShrink: 0 }}>
      {live && <Box component="span" sx={{
        position: 'absolute', inset: 0, borderRadius: '50%', bgcolor: color,
        animation: `${pulse} 1.8s ease-out infinite`,
      }} />}
      <Box component="span" sx={{ position: 'absolute', inset: 0, borderRadius: '50%', bgcolor: color }} />
    </Box>
  )
}

/**
 * A number that rolls from its last reading to the new one, like a mechanical
 * counter. Screen readers get the final value only; the rolling digits are
 * hidden from them so a poll does not announce every frame.
 */
export function CountUp({ value, format = (n: number) => n.toLocaleString(), duration = 700 }: {
  value: number
  format?: (n: number) => string
  duration?: number
}) {
  const { motionOff } = usePrefs()
  const animate = !motionOff && typeof requestAnimationFrame === 'function'
  const [shown, setShown] = useState(animate ? 0 : value)
  const from = useRef(animate ? 0 : value)

  useEffect(() => {
    if (!animate) { setShown(value); from.current = value; return }
    const start = performance.now()
    const a = from.current
    let raf = 0
    const step = (now: number) => {
      const p = Math.min(1, (now - start) / duration)
      const eased = 1 - Math.pow(1 - p, 3)
      const v = Math.round(a + (value - a) * eased)
      setShown(v)
      from.current = v
      if (p < 1) raf = requestAnimationFrame(step)
    }
    raf = requestAnimationFrame(step)
    return () => cancelAnimationFrame(raf)
  }, [value, animate, duration])

  return (
    <>
      <Box component="span" aria-hidden>{format(shown)}</Box>
      <Box component="span" sx={visuallyHidden}>{format(value)}</Box>
    </>
  )
}

const visuallyHidden = {
  position: 'absolute', width: 1, height: 1, p: 0, m: -1, overflow: 'hidden',
  clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', border: 0,
} as const

/**
 * A progress bar that shows a light sweeping along the filled part while the
 * scan is still working, so a running bar and a stalled one look different even
 * when the percentage has not moved.
 */
export function ScanProgress({ active, height = 6, sx, ...rest }: LinearProgressProps & { active: boolean; height?: number }) {
  return (
    <LinearProgress {...rest} sx={{
      height, borderRadius: height / 2,
      ...(active ? {
        '& .MuiLinearProgress-bar': { overflow: 'hidden' },
        '& .MuiLinearProgress-bar::after': {
          content: '""', position: 'absolute', top: 0, bottom: 0, left: 0, width: '25%',
          background: 'linear-gradient(90deg, transparent, rgba(255,255,255,.55), transparent)',
          animation: `${sweep} 1.6s linear infinite`,
        },
      } : {}),
      ...sx,
    }} />
  )
}

/** A radar sweep with a blinking cursor: the loading state of a scanner. */
export function RadarLoader({ label, size = 56 }: { label?: string; size?: number }) {
  return (
    <Box role="progressbar" aria-busy="true" aria-label={label}
      sx={{ p: 6, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.5 }}>
      <Box sx={{
        width: size, height: size, borderRadius: '50%', position: 'relative', overflow: 'hidden',
        border: 1, borderColor: 'primary.main',
        backgroundImage: t => `radial-gradient(circle, transparent 32%, ${t.palette.divider} 33%, transparent 34%, transparent 65%, ${t.palette.divider} 66%, transparent 67%)`,
      }}>
        <Box sx={{
          position: 'absolute', inset: 0, borderRadius: '50%',
          background: t => `conic-gradient(from 0deg, transparent 0deg, transparent 280deg, ${t.palette.primary.main} 360deg)`,
          opacity: .55, animation: `${spin} 1.4s linear infinite`,
        }} />
        <Box sx={{ position: 'absolute', top: '50%', left: '50%', width: 4, height: 4, m: '-2px', borderRadius: '50%', bgcolor: 'primary.main' }} />
      </Box>
      {label && (
        <Box sx={{ fontFamily: t => t.typography.overline.fontFamily, fontSize: 11, letterSpacing: '.14em', textTransform: 'uppercase', color: 'text.secondary' }}>
          {label}
          <Box component="span" aria-hidden sx={{ animation: `${blink} 1s steps(1) infinite` }}>▌</Box>
        </Box>
      )}
    </Box>
  )
}
