import { useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import Card from '@mui/material/Card'
import Link from '@mui/material/Link'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import { useTheme } from '@mui/material/styles'
import KeyGraph from '../components/KeyGraph'
import SeverityChip from '../components/SeverityChip'
import StatCard from '../components/StatCard'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { useBulkTriage } from '../hooks/useTriage'
import { href } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { FindingStatus, SharedKey } from '../types'

// Credentials found in more than one repository.
//
// The graph is the picture; the list beside it is the same data as something a
// keyboard and a screen reader can use, and choosing a key in either selects it
// in both. Every number here comes from the server's grouping by fingerprint —
// nothing is inferred from previews, which collide for unrelated keys.

const MIN_REPOS = [2, 3, 5, 10]

const owners = (k: SharedKey) => new Set(k.repositories.map(r => r.owner))

export default function KeySpreadPage() {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const [minRepos, setMinRepos] = useState(2)
  const [selected, setSelected] = useState<string | null>(null)
  const res = useResource(() => api.sharedKeys(minRepos), [api, minRepos], 30000)

  const keys = res.data?.keys ?? []
  const total = res.data?.total ?? keys.length
  const stats = useMemo(() => {
    const repos = new Set<string>(), owns = new Set<string>()
    for (const k of keys) for (const r of k.repositories) { repos.add(r.id); owns.add(r.owner) }
    return {
      repos: repos.size, owners: owns.size,
      crossOwner: keys.filter(k => owners(k).size > 1).length,
      atHead: keys.filter(k => k.repositories.some(r => r.reachable_from_head && r.status === 'open')).length,
    }
  }, [keys])
  const current = keys.find(k => k.key_id === selected) ?? null
  const bulk = useBulkTriage(res.reload)

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />

  return (
    <>
      <PageHeader title={t('spread.title')} subtitle={t('spread.subtitle')} actions={
        <TextField select size="small" label={t('spread.minRepos')} value={minRepos} sx={{ minWidth: 180 }}
          onChange={e => { setMinRepos(Number(e.target.value)); setSelected(null) }}>
          {MIN_REPOS.map(n => <MenuItem key={n} value={n}>{t('spread.atLeast', { n })}</MenuItem>)}
        </TextField>
      } />

      {!keys.length ? (
        <Card><Empty art="safe" size={200} title={t('spread.empty')} body={t('spread.emptyBody')} /></Card>
      ) : (
        <>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 2, mb: 2 }}>
            <StatCard label={t('spread.keys')} value={total} accent={ch.severity.high}
              hint={total > keys.length ? t('spread.showing', { n: keys.length }) : undefined} />
            <StatCard label={t('spread.crossOwner')} value={stats.crossOwner} hint={t('spread.crossOwnerHint')} accent={ch.severity.critical} />
            <StatCard label={t('spread.atHead')} value={stats.atHead} hint={t('spread.atHeadHint')} />
            <StatCard label={t('spread.reach')} value={`${stats.repos} / ${stats.owners}`} hint={t('spread.reachHint')} />
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '1fr 340px' }, gap: 2, alignItems: 'start' }}>
            <Card sx={{ p: 1, overflow: 'hidden' }}>
              <KeyGraph keys={keys} selected={selected} onSelect={setSelected} label={t('spread.graphLabel', { n: keys.length })} />
              <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap', px: 1.5, py: 1, fontSize: 12, color: 'text.secondary' }}>
                <Legend shape="square" text={t('spread.legend.owner')} />
                <Legend shape="circle" text={t('spread.legend.repo')} />
                <Legend shape="diamond" color={ch.severity.critical} text={t('spread.legend.key')} />
                <Legend shape="line" color={ch.severity.critical} text={t('spread.legend.head')} />
                <Legend shape="dash" text={t('spread.legend.closed')} />
              </Box>
            </Card>

            <Card sx={{ maxHeight: 600, overflow: 'auto' }}>
              {current ? <KeyDetail k={current} onBack={() => setSelected(null)} busy={bulk.running}
                onMark={s => bulk.run(current.repositories.map(r => ({ id: r.finding_id, status: r.status })), s)} /> : (
                <Box component="ul" aria-label={t('spread.listLabel')} sx={{ listStyle: 'none', m: 0, p: 0 }}>
                  {keys.map(k => (
                    <Box component="li" key={k.key_id}>
                      <Box component="button" onClick={() => setSelected(k.key_id)} sx={{
                        all: 'unset', boxSizing: 'border-box', display: 'flex', alignItems: 'center', gap: 1.25, width: '100%',
                        px: 2, py: 1.25, cursor: 'pointer', borderBottom: 1, borderColor: 'divider',
                        '&:hover, &:focus-visible': { bgcolor: 'action.hover' },
                        '&:focus-visible': { outline: t => `2px solid ${t.palette.primary.main}`, outlineOffset: -2 },
                      }}>
                        <SeverityChip severity={k.severity} />
                        <Box sx={{ minWidth: 0, flex: 1 }}>
                          <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: 12.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {k.masked_preview}
                          </Box>
                          <Box sx={{ fontSize: 11.5, color: 'text.secondary' }}>
                            {k.provider || t('findings.unmapped')} · {t('spread.inRepos', { n: k.repositories.length, o: owners(k).size })}
                          </Box>
                        </Box>
                      </Box>
                    </Box>
                  ))}
                </Box>
              )}
            </Card>
          </Box>
        </>
      )}
    </>
  )
}

// Both decisions are claims about every copy at once, so both are confirmed —
// the same rule the bulk bar applies. A value pasted into many unrelated
// owners' repositories is usually boilerplate the generic pass mistook for a
// secret, which is why false positive sits beside rotated here.
const KEY_ACTIONS: FindingStatus[] = ['rotated', 'false_positive']

function KeyDetail({ k, onBack, onMark, busy }: { k: SharedKey; onBack: () => void; onMark: (s: FindingStatus) => void; busy: boolean }) {
  const { t } = useT()
  const [confirm, setConfirm] = useState<FindingStatus | null>(null)
  const n = k.repositories.length
  return (
    <Box sx={{ p: 2 }}>
      <Link component="button" onClick={onBack} underline="hover" sx={{ fontSize: 12.5, mb: 1.5 }}>← {t('spread.back')}</Link>
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: .5 }}>
        <SeverityChip severity={k.severity} />
        <Box sx={{ fontWeight: 700 }}>{k.provider || t('findings.unmapped')}</Box>
      </Box>
      <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: 13, mb: .5, wordBreak: 'break-all' }}>{k.masked_preview}</Box>
      <Box sx={{ fontSize: 12, color: 'text.secondary', mb: 2 }}>{k.rule_id} · {t('spread.keyId')} {k.key_id}</Box>
      <Box sx={{ fontSize: 13, mb: 1.5, lineHeight: 1.55 }}>{t('spread.oneRotation', { n })}</Box>
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
        {KEY_ACTIONS.map(s => (
          <Button key={s} size="small" variant="outlined" disabled={busy} onClick={() => setConfirm(s)}>
            {t('spread.markAll', { status: t(`status.${s}`) })}
          </Button>
        ))}
      </Box>
      <Dialog open={!!confirm} onClose={() => setConfirm(null)}>
        <DialogTitle>{confirm && t('bulk.confirmTitle', { n, status: t(`status.${confirm}`) })}</DialogTitle>
        <DialogContent sx={{ fontSize: 14 }}>{t('spread.markAllBody')}</DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirm(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={() => { if (confirm) onMark(confirm); setConfirm(null) }}>{t('common.confirm')}</Button>
        </DialogActions>
      </Dialog>
      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {k.repositories.map(r => (
          <Box component="li" key={r.finding_id} sx={{ py: .75, borderTop: 1, borderColor: 'divider', fontSize: 13 }}>
            <Link href={href('findings', undefined, { repository_id: r.id, finding: r.finding_id })} underline="hover"
              sx={{ fontFamily: '"IBM Plex Mono", monospace' }}>
              {r.owner}/{r.name}
            </Link>
            <Box sx={{ fontSize: 11.5, color: 'text.secondary' }}>
              {t(`status.${r.status}`)} · {r.reachable_from_head ? t('findings.atHead') : t('findings.history')}
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  )
}

function Legend({ shape, color, text }: { shape: 'square' | 'circle' | 'diamond' | 'line' | 'dash'; color?: string; text: string }) {
  const theme = useTheme()
  const c = color ?? theme.palette.text.secondary
  return (
    <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: .75 }}>
      <svg width={16} height={12} aria-hidden>
        {shape === 'square' && <rect x={3} y={1} width={10} height={10} rx={1.5} fill={theme.palette.text.primary} />}
        {shape === 'circle' && <circle cx={8} cy={6} r={4} fill="none" stroke={c} strokeWidth={1.5} />}
        {shape === 'diamond' && <polygon points="8,0 14,6 8,12 2,6" fill={c} />}
        {shape === 'line' && <line x1={0} y1={6} x2={16} y2={6} stroke={c} strokeWidth={1.6} />}
        {shape === 'dash' && <line x1={0} y1={6} x2={16} y2={6} stroke={c} strokeWidth={1.4} strokeDasharray="3 3" />}
      </svg>
      {text}
    </Box>
  )
}
