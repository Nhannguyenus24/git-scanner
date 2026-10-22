import Box from '@mui/material/Box'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import type { Finding, Severity } from '../types'

const SEV_COLOR: Record<Severity, 'error' | 'warning' | 'default'> = {
  critical: 'error',
  high:     'error',
  medium:   'warning',
  low:      'default',
  info:     'default',
}

interface Props {
  findings: Finding[]
  loading:  boolean
}

export default function FindingsTable({ findings, loading }: Props) {
  if (loading) {
    return (
      <Box sx={{ p: 6, textAlign: 'center' }}>
        <CircularProgress size={32} />
      </Box>
    )
  }

  if (!findings.length) {
    return (
      <Box sx={{ p: 4, textAlign: 'center', color: 'text.secondary' }}>
        <Typography variant="body2">No findings yet — results appear as the scan progresses.</Typography>
      </Box>
    )
  }

  return (
    <TableContainer>
      <Table size="small" stickyHeader>
        <TableHead>
          <TableRow>
            <TableCell>Rule</TableCell>
            <TableCell>Provider</TableCell>
            <TableCell>Severity</TableCell>
            <TableCell>Preview</TableCell>
            <TableCell>File</TableCell>
            <TableCell>Reachable</TableCell>
            <TableCell>Status</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {findings.map(f => (
            <TableRow key={f.id} hover>
              <TableCell sx={{ fontFamily: 'monospace', fontSize: 12 }}>{f.rule_id}</TableCell>

              <TableCell>
                <Chip label={f.provider || '—'} size="small" variant="outlined" />
              </TableCell>

              <TableCell>
                <Chip label={f.severity} size="small" color={SEV_COLOR[f.severity]} />
              </TableCell>

              <TableCell sx={{ fontFamily: 'monospace', fontSize: 12, color: 'primary.main' }}>
                {f.masked_preview}
              </TableCell>

              <TableCell>
                <Tooltip title={f.file_path} placement="top">
                  <Box sx={{ fontFamily: 'monospace', fontSize: 12, cursor: 'default' }}>
                    {f.file_path.split('/').pop()}:{f.line_number}
                  </Box>
                </Tooltip>
              </TableCell>

              <TableCell>
                <Chip
                  label={f.reachable_from_head ? 'live' : 'history'}
                  size="small"
                  color={f.reachable_from_head ? 'error' : 'default'}
                  variant="outlined"
                />
              </TableCell>

              <TableCell>
                <Chip
                  label={f.status}
                  size="small"
                  variant="outlined"
                  color={f.status === 'open' ? 'error' : f.status === 'rotated' ? 'success' : 'default'}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}
