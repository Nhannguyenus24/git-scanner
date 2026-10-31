import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Link from '@mui/material/Link'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import Typography from '@mui/material/Typography'
import DownloadIcon from '@mui/icons-material/Download'
import PrintIcon from '@mui/icons-material/Print'
import SeverityChip from '../components/SeverityChip'
import { Empty, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { useT } from '../i18n'
import { useSettings } from '../store/settings'
import type { Count, SharedReport } from '../types'

const MONO = '"IBM Plex Mono", monospace'
const when = (iso?: string) => (iso ? new Date(iso).toLocaleString() : '—')

function Kpi({ n, label, accent }: { n: number; label: string; accent?: boolean }) {
  return (
    <Card variant="outlined" sx={{ p: 1.5 }}>
      <Box sx={{ fontSize: 24, fontWeight: 800, color: accent && n ? 'error.main' : 'text.primary' }}>{n}</Box>
      <Box sx={{ fontSize: 12.5, color: 'text.secondary' }}>{label}</Box>
    </Card>
  )
}

function Breakdown({ title, rows, severity }: { title: string; rows: Count[]; severity?: boolean }) {
  if (!rows.length) return null
  return (
    <Card variant="outlined" sx={{ p: 1.5 }}>
      <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: 1 }}>{title}</Box>
      {rows.map(c => (
        <Box key={c.key} sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', py: .35, fontSize: 13.5 }}>
          {severity ? <SeverityChip severity={c.key as never} /> : <span>{c.key}</span>}
          <Box component="span" sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: 600 }}>{c.count}</Box>
        </Box>
      ))}
    </Card>
  )
}

/**
 * The page a share link opens. It stands apart from the dashboard — no
 * navigation, no other data — because whoever holds the link should see this
 * repository's report and nothing else.
 */
export default function SharedReportPage({ token }: { token: string }) {
  const { api } = useSettings()
  const { t } = useT()
  const res = useResource<SharedReport>(() => api.sharedReport(token), [api, token])

  if (res.loading) return <Loading />
  if (!res.data) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: 3 }}>
        <Empty art="no-data" title={t('shared.gone')} body={t('shared.goneBody')} />
      </Box>
    )
  }
  const r = res.data
  const s = r.summary

  return (
    <Box component="main" sx={{ maxWidth: 1040, mx: 'auto', px: 2, py: 3 }}>
      <Box sx={{ fontSize: 12, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'error.main' }}>
        {t('shared.kicker')}
      </Box>
      <Typography component="h1" sx={{ fontSize: 26, fontWeight: 800, mt: .5 }}>{r.repo}</Typography>
      <Box sx={{ fontSize: 13, color: 'text.secondary', mt: .5 }}>
        <Link href={r.repo_url} target="_blank" rel="noreferrer noopener">{r.repo_url}</Link>
        {' · '}{t('shared.scanned', { when: when(r.last_scanned_at) })}
        {' · '}{t('shared.validUntil', { when: when(r.expires_at) })}
      </Box>

      <Box sx={{ display: 'flex', gap: 1, mt: 2, flexWrap: 'wrap' }}>
        <Button variant="outlined" startIcon={<DownloadIcon />} href={api.sharedReportUrl(token, 'md')}>{t('shared.downloadMd')}</Button>
        <Button variant="outlined" startIcon={<PrintIcon />} href={api.sharedReportUrl(token, 'html')} target="_blank" rel="noreferrer noopener">
          {t('shared.printable')}
        </Button>
      </Box>

      <Alert severity={r.detail === 'full' ? 'warning' : 'info'} sx={{ mt: 2 }}>
        {t(r.detail === 'full' ? 'shared.noticeFull' : 'shared.noticeSummary')}
      </Alert>

      <Box sx={{ display: 'grid', gap: 1.25, mt: 2, gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(5, 1fr)' } }}>
        <Kpi n={s.total} label={t('shared.kpi.total')} />
        <Kpi n={s.open} label={t('shared.kpi.open')} accent />
        <Kpi n={s.live} label={t('shared.kpi.live')} accent />
        <Kpi n={s.on_head} label={t('shared.kpi.onHead')} />
        <Kpi n={s.files} label={t('shared.kpi.files')} />
      </Box>
      <Box sx={{ display: 'grid', gap: 1.25, mt: 1.25, gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' } }}>
        <Breakdown title={t('shared.bySeverity')} rows={s.by_severity} severity />
        <Breakdown title={t('shared.byProvider')} rows={s.by_provider} />
        <Breakdown title={t('shared.byStatus')} rows={s.by_status} />
      </Box>

      {r.findings.length > 0 && (
        <Card variant="outlined" sx={{ mt: 2 }}>
          <Typography component="h2" sx={{ fontWeight: 700, px: 2, pt: 1.5 }}>{t('shared.findings')}</Typography>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t('shared.col.severity')}</TableCell>
                  <TableCell>{t('shared.col.provider')}</TableCell>
                  <TableCell>{t('shared.col.location')}</TableCell>
                  <TableCell>{t('shared.col.value')}</TableCell>
                  <TableCell>{t('shared.col.commit')}</TableCell>
                  <TableCell>{t('shared.col.status')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {r.findings.map((f, i) => (
                  <TableRow key={i}>
                    <TableCell>
                      <SeverityChip severity={f.severity} />
                      {f.live && <Box sx={{ fontSize: 11, fontWeight: 700, color: 'error.main', mt: .25 }}>{t('shared.live')}</Box>}
                    </TableCell>
                    <TableCell>{f.provider}<Box sx={{ fontFamily: MONO, fontSize: 11.5, color: 'text.secondary' }}>{f.rule}</Box></TableCell>
                    <TableCell sx={{ fontFamily: MONO, fontSize: 12.5, wordBreak: 'break-all' }}>
                      {f.location}
                      {!f.on_head && <Box sx={{ fontFamily: 'inherit', fontSize: 11.5, color: 'text.secondary' }}>{t('shared.historyOnly')}</Box>}
                    </TableCell>
                    <TableCell sx={{ fontFamily: MONO, fontSize: 12.5 }}>{f.masked}</TableCell>
                    <TableCell sx={{ fontFamily: MONO, fontSize: 12.5 }}>
                      {f.link ? <Link href={f.link} target="_blank" rel="noreferrer noopener">{f.commit}</Link> : f.commit}
                      {f.committed_at && <Box sx={{ fontSize: 11.5, color: 'text.secondary' }}>{f.committed_at}</Box>}
                    </TableCell>
                    <TableCell>{t(`status.${f.status}`)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Card>
      )}

      {r.guide.length > 0 && (
        <Card variant="outlined" sx={{ mt: 2, p: 2 }}>
          <Typography component="h2" sx={{ fontWeight: 700, mb: 1 }}>{t('shared.fix')}</Typography>
          <Box component="ol" sx={{ m: 0, pl: 2.5 }}>
            {r.guide.map((g, i) => (
              <Box component="li" key={i} sx={{ mb: 1.5 }}>
                <Box sx={{ fontWeight: 600 }}>{g.title}</Box>
                {g.text.map((p, j) => <Box key={j} sx={{ fontSize: 13.5, color: 'text.secondary', mt: .25 }}>{p}</Box>)}
                {g.commands.length > 0 && (
                  <Box component="pre" sx={{ fontFamily: MONO, fontSize: 12, bgcolor: 'action.hover', p: 1, borderRadius: 1, whiteSpace: 'pre-wrap', wordBreak: 'break-all', mt: .75, mb: 0 }}>
                    {g.commands.join('\n')}
                  </Box>
                )}
              </Box>
            ))}
          </Box>
        </Card>
      )}

      <Box sx={{ fontSize: 12, color: 'text.disabled', mt: 3 }}>{t('shared.footer', { when: when(r.generated_at) })}</Box>
    </Box>
  )
}
