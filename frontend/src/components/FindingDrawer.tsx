import { useEffect, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import Divider from '@mui/material/Divider'
import Drawer from '@mui/material/Drawer'
import IconButton from '@mui/material/IconButton'
import Link from '@mui/material/Link'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import Tooltip from '@mui/material/Tooltip'
import CloseIcon from '@mui/icons-material/Close'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import VisibilityIcon from '@mui/icons-material/VisibilityOutlined'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined'
import SeverityChip from './SeverityChip'
import { commitLink, fileLink, timeAgo } from '../format'
import { href } from '../hooks/useRoute'
import { useT } from '../i18n'
import { STATUSES, type Finding, type FindingStatus } from '../types'

// Metadata keys already shown in their own section, so the raw table below
// lists only what is left.
const SHOWN = new Set([
  'provider_name', 'category', 'probe_status', 'probe_owner', 'probe_account', 'probe_team',
  'probe_rate_remaining', 'probe_rate_limit', 'probe_tokens_remaining', 'probe_scopes',
  'probe_country', 'probe_currency',
])

// Quota and account facts the probe read from the provider's own response, in
// the order they are worth reading: how much of the allowance is left, then what
// the key can do, then where the account lives.
const QUOTA_ROWS: { key: string; label: string; limitKey?: string }[] = [
  { key: 'probe_rate_remaining', label: 'detail.rateRemaining', limitKey: 'probe_rate_limit' },
  { key: 'probe_tokens_remaining', label: 'detail.tokensRemaining' },
  { key: 'probe_scopes', label: 'detail.scopes' },
  { key: 'probe_country', label: 'detail.country' },
  { key: 'probe_currency', label: 'detail.currency' },
]

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '120px 1fr', gap: 1.5, py: .75, fontSize: 13 }}>
      <Box sx={{ color: 'text.secondary' }}>{label}</Box>
      <Box sx={{ minWidth: 0, wordBreak: 'break-word' }}>{children}</Box>
    </Box>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Box sx={{ px: 3, py: 2 }}>
      <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: 1 }}>{title}</Box>
      {children}
    </Box>
  )
}

const copy = (s: string) => navigator.clipboard?.writeText(s)

/** Everything known about one finding, in a side panel, so the table row can
 *  stay one line tall. The unmasked value is hidden until asked for, even when
 *  the server sent it: a screen share should not leak it by default. */
export default function FindingDrawer({ finding: f, onClose, onStatus }: {
  finding: Finding | null
  onClose: () => void
  onStatus: (f: Finding, s: FindingStatus) => void
}) {
  const { t } = useT()
  const [reveal, setReveal] = useState(false)
  useEffect(() => setReveal(false), [f?.id])

  const link = f ? fileLink(f.repository.url, f.file_path, f.line_number, f.commit_sha, f.reachable_from_head) : ''
  const probe = f?.metadata?.probe_status
  const who = f && (f.metadata.probe_owner || f.metadata.probe_account || f.metadata.probe_team)
  const extra = f ? Object.entries(f.metadata ?? {}).filter(([k]) => !SHOWN.has(k)) : []

  return (
    <Drawer anchor="right" open={!!f} onClose={onClose} PaperProps={{ sx: { width: { xs: '100%', sm: 520 } } }}>
      {f && (
        <>
          <Box sx={{ px: 3, pt: 2.5, pb: 2, display: 'flex', gap: 1, alignItems: 'flex-start' }}>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Box sx={{ display: 'flex', gap: .75, alignItems: 'center', flexWrap: 'wrap' }}>
                <SeverityChip severity={f.severity} />
                {probe === 'live' && <Chip size="small" color="error" label={t('probe.live')} />}
                {f.secret && <Chip size="small" color="error" variant="outlined" label={t('settings.unmasked')} />}
              </Box>
              <Box sx={{ fontSize: 20, fontWeight: 700, mt: 1 }}>{f.metadata.provider_name || f.provider || t('findings.unmapped')}</Box>
              <Box sx={{ fontFamily: 'monospace', fontSize: 12, color: 'text.secondary' }}>{f.rule_id}</Box>
            </Box>
            <IconButton onClick={onClose} aria-label={t('common.close')}><CloseIcon /></IconButton>
          </Box>
          <Divider />

          <Section title={t('detail.credential')}>
            <Field label={t('detail.masked')}>
              <Box component="code" sx={{ fontSize: 13, color: 'primary.main' }}>{f.masked_preview}</Box>
            </Field>
            {f.secret && (
              <Field label={t('detail.value')}>
                <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: .5 }}>
                  <Box component="code" sx={{
                    flex: 1, fontSize: 12.5, p: 1, borderRadius: 1, bgcolor: 'action.hover', wordBreak: 'break-all',
                    color: reveal ? 'error.main' : 'text.disabled', filter: reveal ? 'none' : 'blur(5px)', userSelect: reveal ? 'text' : 'none',
                  }}>{f.secret}</Box>
                  <Tooltip title={t(reveal ? 'detail.hide' : 'detail.show')}>
                    <IconButton size="small" onClick={() => setReveal(r => !r)}>{reveal ? <VisibilityOffIcon fontSize="small" /> : <VisibilityIcon fontSize="small" />}</IconButton>
                  </Tooltip>
                  <Tooltip title={t('common.copy')}>
                    <IconButton size="small" onClick={() => copy(f.secret!)}><ContentCopyIcon fontSize="small" /></IconButton>
                  </Tooltip>
                </Box>
              </Field>
            )}
            {f.metadata.category && <Field label={t('detail.category')}>{f.metadata.category}</Field>}
            {probe && (
              <Field label={t('detail.liveness')}>
                {t(`probe.${probe}`)}{who && <Box component="span" sx={{ color: 'text.secondary' }}> · {who}</Box>}
              </Field>
            )}
          </Section>

          {QUOTA_ROWS.some(r => f.metadata?.[r.key]) && (
            <>
              <Divider />
              <Section title={t('detail.quota')}>
                {QUOTA_ROWS.filter(r => f.metadata?.[r.key]).map(r => (
                  <Field key={r.key} label={t(r.label)}>
                    <Box component="span" sx={{ fontVariantNumeric: 'tabular-nums' }}>{f.metadata[r.key]}</Box>
                    {r.limitKey && f.metadata[r.limitKey] && (
                      <Box component="span" sx={{ color: 'text.secondary' }}> / {f.metadata[r.limitKey]}</Box>
                    )}
                  </Field>
                ))}
                <Box sx={{ fontSize: 11, color: 'text.disabled', mt: 1 }}>{t('detail.quotaNote')}</Box>
              </Section>
            </>
          )}

          <Divider />

          <Section title={t('detail.where')}>
            <Field label={t('findings.col.repository')}>
              <Link href={href('repositories', f.repository.id)} underline="hover" onClick={onClose}>{f.repository.owner}/{f.repository.name}</Link>
            </Field>
            <Field label={t('detail.file')}>
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: .5 }}>
                <Box component="code" sx={{ fontSize: 12.5, wordBreak: 'break-all' }}>{f.file_path}{f.line_number > 0 && `:${f.line_number}`}</Box>
                <Tooltip title={t('common.copy')}><IconButton size="small" sx={{ p: .25 }} onClick={() => copy(f.file_path)}><ContentCopyIcon sx={{ fontSize: 14 }} /></IconButton></Tooltip>
              </Box>
            </Field>
            <Field label={t('findings.col.exposure')}>{f.reachable_from_head ? t('findings.atHeadTip') : t('findings.historyTip')}</Field>
            {f.commit_sha ? (
              <>
                <Field label={t('detail.commit')}>
                  <Link href={commitLink(f.repository.url, f.commit_sha)} target="_blank" rel="noreferrer" sx={{ fontFamily: 'monospace' }}>{f.commit_sha.slice(0, 12)}</Link>
                </Field>
                {f.author_name && <Field label={t('detail.author')}>{f.author_name}</Field>}
                {f.committed_at && <Field label={t('detail.committed')}>{new Date(f.committed_at).toLocaleString()} · {timeAgo(f.committed_at)}</Field>}
              </>
            ) : <Field label={t('detail.commit')}><Box sx={{ color: 'text.disabled' }}>{t('findings.commitUnknown')}</Box></Field>}
            <Field label={t('findings.col.detected')}>{new Date(f.detected_at).toLocaleString()}</Field>
            {link && (
              <Button size="small" variant="outlined" endIcon={<OpenInNewIcon />} href={link} target="_blank" rel="noreferrer" sx={{ mt: 1.5 }}>
                {t('detail.openFile')}
              </Button>
            )}
          </Section>

          {extra.length > 0 && (
            <>
              <Divider />
              <Section title={t('detail.metadata')}>
                {extra.map(([k, v]) => <Field key={k} label={k}><Box component="code" sx={{ fontSize: 12 }}>{v}</Box></Field>)}
              </Section>
            </>
          )}
          <Divider />

          <Section title={t('findings.col.status')}>
            <TextField select size="small" value={f.status} onChange={e => onStatus(f, e.target.value as FindingStatus)} sx={{ minWidth: 220 }}>
              {STATUSES.map(s => <MenuItem key={s} value={s}>{t(`status.${s}`)}</MenuItem>)}
            </TextField>
          </Section>
        </>
      )}
    </Drawer>
  )
}
