import { useState, useEffect } from 'react'
import Alert from '@mui/material/Alert'
import Badge from '@mui/material/Badge'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import Divider from '@mui/material/Divider'
import LinearProgress from '@mui/material/LinearProgress'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import Tab from '@mui/material/Tab'
import Tabs from '@mui/material/Tabs'
import Typography from '@mui/material/Typography'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import StopIcon from '@mui/icons-material/Stop'
import FindingsTable from './FindingsTable'
import StateBadge from './StateBadge'
import { createClient } from '../api/client'
import { TERMINAL } from '../types'
import type { LocalJob, Finding } from '../types'

const KIND_ICON: Record<string, string> = { repo: '📦', user: '👤', org: '🏢', unknown: '🔍' }

interface StatTileProps { label: string; value: number; color?: string }

function StatTile({ label, value, color }: StatTileProps) {
  return (
    <Paper variant="outlined" sx={{ px: 2, py: 1, textAlign: 'center', minWidth: 80, borderColor: 'divider', flex: 1 }}>
      <Typography variant="h6" sx={{ fontWeight: 700, color: color ?? 'text.primary', fontVariantNumeric: 'tabular-nums' }}>
        {value}
      </Typography>
      <Typography variant="caption" color="text.secondary">{label}</Typography>
    </Paper>
  )
}

interface Props {
  job:      LocalJob
  baseUrl:  string
  onAction: (action: 'pause' | 'resume' | 'cancel', id: string) => Promise<void>
}

export default function JobDetail({ job, baseUrl, onAction }: Props) {
  const [tab,      setTab]      = useState(0)
  const [findings, setFindings] = useState<Finding[]>([])
  const [fLoading, setFLoading] = useState(false)
  const [acting,   setActing]   = useState<string>('')

  // reset when job changes
  useEffect(() => { setTab(0); setFindings([]) }, [job.id])

  // load findings when Findings tab is active
  useEffect(() => {
    if (tab !== 1) return
    setFLoading(true)
    createClient(baseUrl)
      .getFindings(job.id)
      .then(r => setFindings(r.findings))
      .catch(() => {})
      .finally(() => setFLoading(false))
  }, [tab, job.id, job.findings_count, baseUrl])

  const act = async (action: 'pause' | 'resume' | 'cancel') => {
    setActing(action)
    try { await onAction(action, job.id) } finally { setActing('') }
  }

  const pct       = job.repos_total > 0 ? Math.round(job.progress * 100) : null
  const isRunning = job.state === 'running'
  const isPaused  = job.state === 'paused'
  const terminal  = TERMINAL.has(job.state)

  return (
    <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2.5, height: '100%', overflow: 'auto' }}>

      {/* header */}
      <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <Typography sx={{ fontSize: 22, mt: '2px' }}>{KIND_ICON[job.target.kind]}</Typography>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="h6" sx={{ fontWeight: 700, wordBreak: 'break-all', lineHeight: 1.25 }}>
            {job.target.value}
          </Typography>
          <Typography variant="caption" sx={{ fontFamily: 'monospace', color: 'text.secondary' }}>
            {job.id}
          </Typography>
        </Box>
        <StateBadge state={job.state} />
      </Box>

      {/* progress */}
      {!terminal && (
        <Box>
          {pct !== null
            ? <LinearProgress variant="determinate" value={pct} sx={{ height: 6, borderRadius: 3, mb: 0.75 }} />
            : <LinearProgress sx={{ height: 6, borderRadius: 3, mb: 0.75 }} />}
          <Typography variant="caption" color="text.secondary">
            {pct !== null ? `${pct}% · ` : ''}
            {job.repos_scanned} / {job.repos_total || '?'} repos — {job.blobs_scanned.toLocaleString()} blobs scanned
          </Typography>
        </Box>
      )}

      {/* stats */}
      <Stack direction="row" spacing={1}>
        <StatTile label="Repos"    value={job.repos_total} />
        <StatTile label="Scanned"  value={job.repos_scanned}  color="success.main" />
        <StatTile label="Blobs"    value={job.blobs_scanned} />
        <StatTile label="Findings" value={job.findings_count} color={job.findings_count > 0 ? 'error.main' : undefined} />
      </Stack>

      {/* error */}
      {job.error && (
        <Alert severity="error" sx={{ fontFamily: 'monospace', fontSize: 12 }}>{job.error}</Alert>
      )}

      {/* actions */}
      {!terminal && (
        <Stack direction="row" spacing={1} flexWrap="wrap">
          {isRunning && (
            <Button variant="outlined" color="warning" size="small" startIcon={acting === 'pause' ? <CircularProgress size={12} /> : <PauseIcon />}
              disabled={!!acting} onClick={() => act('pause')}>Pause</Button>
          )}
          {isPaused && (
            <Button variant="outlined" color="success" size="small" startIcon={acting === 'resume' ? <CircularProgress size={12} /> : <PlayArrowIcon />}
              disabled={!!acting} onClick={() => act('resume')}>Resume</Button>
          )}
          <Button variant="outlined" color="error" size="small" startIcon={acting === 'cancel' ? <CircularProgress size={12} /> : <StopIcon />}
            disabled={!!acting} onClick={() => act('cancel')}>Cancel</Button>
        </Stack>
      )}

      <Divider />

      {/* tabs */}
      <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <Tabs value={tab} onChange={(_, v: number) => setTab(v)} textColor="primary" indicatorColor="primary" sx={{ mb: 1 }}>
          <Tab label="Details" />
          <Tab label={
            <Badge badgeContent={job.findings_count} color="error" max={9999}>
              <Box sx={{ pr: job.findings_count > 0 ? 1.5 : 0 }}>Findings</Box>
            </Badge>
          } />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Stack divider={<Divider />}>
              {([
                ['Job ID',       job.id,                           true],
                ['State',        job.state,                        false],
                ['Target kind',  job.target.kind,                  false],
                ['Target',       job.target.value,                 false],
                ['Progress',     pct !== null ? `${pct}%` : '—',  false],
                ['Submitted',    new Date(job.submittedAt).toLocaleString(), false],
              ] as [string, string, boolean][]).map(([k, v, mono]) => (
                <Box key={k} sx={{ display: 'flex', gap: 2, py: 1.25, alignItems: 'baseline' }}>
                  <Typography variant="caption" color="text.secondary"
                    sx={{ minWidth: 100, flexShrink: 0, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '.05em' }}>
                    {k}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: mono ? 'monospace' : undefined, wordBreak: 'break-all' }}>
                    {v}
                  </Typography>
                </Box>
              ))}
            </Stack>
          )}

          {tab === 1 && <FindingsTable findings={findings} loading={fLoading} />}
        </Box>
      </Box>
    </Box>
  )
}
