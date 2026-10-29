import { useState, type MouseEvent } from 'react'
import Button from '@mui/material/Button'
import Divider from '@mui/material/Divider'
import ListItemText from '@mui/material/ListItemText'
import Menu from '@mui/material/Menu'
import MenuItem from '@mui/material/MenuItem'
import Box from '@mui/material/Box'
import DownloadIcon from '@mui/icons-material/FileDownloadOutlined'
import {
  download, exportName, findingsCsv, findingsJson, incidentMarkdown, executiveMarkdown, type ExportScope,
} from '../export'
import { useToast } from '../store/toast'
import { useT } from '../i18n'
import type { Finding, Repository, ScanSummary } from '../types'

// Exports what is loaded, with the filters that produced it written into the
// file. Every format goes through src/export.ts, which builds its rows from an
// explicit column list — the unmasked `secret` is not one of them, in any format.

interface Props {
  findings:      Finding[]
  scope:         ExportScope
  /** Only needed for the executive summary. */
  repositories?: Repository[]
  scans?:        ScanSummary[]
  label?:        string
  size?:         'small' | 'medium'
}

export default function ExportMenu({ findings, scope, repositories, scans, label, size = 'small' }: Props) {
  const { t } = useT()
  const notify = useToast()
  const [anchor, setAnchor] = useState<null | HTMLElement>(null)
  const close = () => setAnchor(null)

  const run = (fn: () => void, what: string) => () => {
    close()
    try {
      fn()
      notify(t('export.done', { what, n: findings.length }))
    } catch (e) {
      // A failed download is silent in most browsers; say so rather than
      // leaving someone waiting for a file that is not coming.
      notify(e instanceof Error ? e.message : t('export.failed'), 'error')
    }
  }

  const disabled = !findings.length

  return (
    <>
      <Button size={size} startIcon={<DownloadIcon />} onClick={(e: MouseEvent<HTMLElement>) => setAnchor(e.currentTarget)}
        disabled={disabled} aria-haspopup="menu">
        {label ?? t('export.action')}
      </Button>
      <Menu anchorEl={anchor} open={!!anchor} onClose={close}>
        <MenuItem disabled sx={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.07em' }}>
          {t('export.scope', { n: findings.length })}
        </MenuItem>
        <MenuItem onClick={run(() => download(exportName('findings', 'csv'), findingsCsv(findings, scope), 'text/csv'), 'CSV')}>
          <ListItemText primary={t('export.csv')} secondary={t('export.csvHint')} />
        </MenuItem>
        <MenuItem onClick={run(() => download(exportName('findings', 'json'), findingsJson(findings, scope), 'application/json'), 'JSON')}>
          <ListItemText primary={t('export.json')} secondary={t('export.jsonHint')} />
        </MenuItem>
        <Divider />
        <MenuItem onClick={run(() => download(exportName('incident', 'md'), incidentMarkdown(findings, scope), 'text/markdown'), 'Markdown')}>
          <ListItemText primary={t('export.incident')} secondary={t('export.incidentHint')} />
        </MenuItem>
        {repositories && (
          <MenuItem onClick={run(() => download(
            exportName('summary', 'md'), executiveMarkdown(findings, repositories, scans ?? [], scope), 'text/markdown'), 'Markdown')}>
            <ListItemText primary={t('export.executive')} secondary={t('export.executiveHint')} />
          </MenuItem>
        )}
        <Box sx={{ px: 2, pt: 1, pb: .5, fontSize: 11, color: 'text.disabled', maxWidth: 300 }}>
          {t('export.maskNote')}
        </Box>
      </Menu>
    </>
  )
}
