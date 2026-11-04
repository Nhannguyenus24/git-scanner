import { useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import AssignmentTurnedInIcon from '@mui/icons-material/AssignmentTurnedIn'
import AccessTimeIcon from '@mui/icons-material/AccessTime'
import BoltIcon from '@mui/icons-material/Bolt'
import FindingsTable from '../components/FindingsTable'
import BulkBar from '../components/BulkBar'
import SeverityChip from '../components/SeverityChip'
import { PageHeader } from '../components/Layout'
import { LoadError, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { useBulkTriage, useTriage } from '../hooks/useTriage'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import { SEVERITIES, type Finding } from '../types'

type Queue = 'all' | 'urgent' | 'stale' | 'history'

function priority(f: Finding): number {
  const severity = (SEVERITIES.length - SEVERITIES.indexOf(f.severity)) * 100
  const exposure = f.reachable_from_head ? 50 : 0
  const live = f.metadata?.probe_status === 'live' ? 100 : 0
  const age = Math.min(30, Math.max(0, Math.floor((Date.now() - new Date(f.detected_at).getTime()) / 86400000)))
  return live + exposure + severity + age
}

function ageDays(f: Finding): number {
  return Math.max(0, Math.floor((Date.now() - new Date(f.detected_at).getTime()) / 86400000))
}

export default function RemediationPage() {
  const { api } = useSettings()
  const { t } = useT()
  const [queue, setQueue] = useState<Queue>('all')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [openId, setOpenId] = useState<string | null>(null)
  const res = useResource(() => api.findings({ status: 'open', limit: 500 }), [api], 10000)
  const triage = useTriage(res.reload)
  const bulk = useBulkTriage(res.reload)

  const findings = useMemo(() => {
    let rows = [...(res.data?.findings ?? [])].sort((a, b) => priority(b) - priority(a))
    if (queue === 'urgent') rows = rows.filter(f => f.reachable_from_head || f.metadata?.probe_status === 'live' || f.severity === 'critical')
    if (queue === 'stale') rows = rows.filter(f => ageDays(f) >= 30)
    if (queue === 'history') rows = rows.filter(f => !f.reachable_from_head)
    return rows
  }, [res.data, queue])

  const counts = useMemo(() => {
    const all = res.data?.findings ?? []
    return {
      all: all.length,
      urgent: all.filter(f => f.reachable_from_head || f.metadata?.probe_status === 'live' || f.severity === 'critical').length,
      stale: all.filter(f => ageDays(f) >= 30).length,
      history: all.filter(f => !f.reachable_from_head).length,
    }
  }, [res.data])

  const selectedFindings = findings.filter(f => selected.has(f.id))
  const toggle = (ids: string[], checked: boolean) => setSelected(current => {
    const next = new Set(current)
    ids.forEach(id => checked ? next.add(id) : next.delete(id))
    return next
  })

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />

  return (
    <>
      <PageHeader title={t('remediation.title')} subtitle={t('remediation.subtitle', { n: findings.length })}
        actions={<Button variant="outlined" onClick={() => navigate(href('findings', undefined, { status: 'open' }))}>{t('remediation.openFullList')}</Button>} />

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2 }}>
        {([
          ['all', t('remediation.all'), counts.all, <AssignmentTurnedInIcon />],
          ['urgent', t('remediation.urgent'), counts.urgent, <BoltIcon />],
          ['stale', t('remediation.stale'), counts.stale, <AccessTimeIcon />],
          ['history', t('remediation.history'), counts.history, <SeverityChip severity="low" />],
        ] as const).map(([id, label, count, icon]) => (
          <Card key={id} component="button" onClick={() => { setQueue(id); setSelected(new Set()) }}
            sx={{ textAlign: 'left', p: 1.5, cursor: 'pointer', border: 1, borderColor: queue === id ? 'primary.main' : 'divider', color: 'inherit' }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', color: queue === id ? 'primary.main' : 'text.secondary' }}>
              <Box sx={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.06em' }}>{label}</Box>
              {icon}
            </Box>
            <Box sx={{ fontSize: 28, fontWeight: 800, lineHeight: 1.2, mt: .75 }}>{count}</Box>
          </Card>
        ))}
      </Box>

      <Card sx={{ p: 1.5, mb: 2, display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <TextField select size="small" label={t('remediation.queue')} value={queue} onChange={e => { setQueue(e.target.value as Queue); setSelected(new Set()) }} sx={{ minWidth: 180 }}>
          <MenuItem value="all">{t('remediation.all')}</MenuItem>
          <MenuItem value="urgent">{t('remediation.urgent')}</MenuItem>
          <MenuItem value="stale">{t('remediation.stale')}</MenuItem>
          <MenuItem value="history">{t('remediation.history')}</MenuItem>
        </TextField>
        <Chip size="small" variant="outlined" label={t('remediation.order')} />
        <Box sx={{ color: 'text.secondary', fontSize: 12, ml: 'auto' }}>{t('remediation.sla')}</Box>
      </Card>

      <Card>
        <FindingsTable findings={findings} onStatus={triage} openId={openId} onOpen={setOpenId}
          selected={selected} onSelect={toggle} emptyMessage={t('remediation.empty')} />
      </Card>
      <BulkBar findings={selectedFindings} running={bulk.running} onClear={() => setSelected(new Set())}
        onStatus={status => bulk.run(selectedFindings.map(f => ({ id: f.id, status: f.status })), status)}
        scope={{ filters: { status: 'open', queue }, total: selectedFindings.length }} />
    </>
  )
}
