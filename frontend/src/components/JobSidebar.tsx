import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Divider from '@mui/material/Divider'
import LinearProgress from '@mui/material/LinearProgress'
import List from '@mui/material/List'
import ListItemButton from '@mui/material/ListItemButton'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import AddIcon from '@mui/icons-material/Add'
import StateBadge from './StateBadge'
import type { LocalJob } from '../types'

const KIND_ICON: Record<string, string> = { repo: '📦', user: '👤', org: '🏢', unknown: '🔍' }

interface Props {
  jobs:         LocalJob[]
  selectedId:   string | null
  onSelect:     (id: string) => void
  onNewScan:    () => void
  onAddById:    (id: string) => void
}

export default function JobSidebar({ jobs, selectedId, onSelect, onNewScan, onAddById }: Props) {
  const [addVal, setAddVal] = useState('')

  const handleAdd = () => {
    const id = addVal.trim()
    if (!id) return
    onAddById(id)
    setAddVal('')
  }

  return (
    <Box sx={{
      width: 260,
      flexShrink: 0,
      display: 'flex',
      flexDirection: 'column',
      bgcolor: 'background.paper',
      borderRight: '1px solid',
      borderColor: 'divider',
      overflow: 'hidden',
      height: '100%',
    }}>
      <Box sx={{ p: 1.5 }}>
        <Button
          fullWidth
          variant="contained"
          startIcon={<AddIcon />}
          onClick={onNewScan}
          sx={{ fontWeight: 700 }}
        >
          New Scan
        </Button>
      </Box>

      {/* add by ID */}
      <Box sx={{ px: 1.5, pb: 1.5, display: 'flex', gap: 1 }}>
        <TextField
          fullWidth
          placeholder="Paste job ID…"
          value={addVal}
          onChange={e => setAddVal(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleAdd() }}
          inputProps={{ style: { fontFamily: 'monospace', fontSize: 11 } }}
        />
        <Button variant="outlined" size="small" onClick={handleAdd} sx={{ flexShrink: 0, minWidth: 0, px: 1.5 }}>
          Add
        </Button>
      </Box>

      <Divider />

      {/* job list */}
      <Box sx={{ flex: 1, overflowY: 'auto' }}>
        {jobs.length === 0 ? (
          <Box sx={{ p: 3, textAlign: 'center', color: 'text.secondary' }}>
            <Typography variant="body2">No scans yet.<br />Click "New Scan" to begin.</Typography>
          </Box>
        ) : (
          <List dense disablePadding sx={{ px: 1, py: 0.5 }}>
            {jobs.map(job => (
              <ListItemButton
                key={job.id}
                selected={job.id === selectedId}
                onClick={() => onSelect(job.id)}
                sx={{ flexDirection: 'column', alignItems: 'flex-start', px: 1.5, py: 1, mb: 0.5 }}
              >
                <Box sx={{ display: 'flex', width: '100%', gap: 1, alignItems: 'center', mb: 0.5 }}>
                  <Typography sx={{ fontSize: 14, lineHeight: 1 }}>
                    {KIND_ICON[job.target.kind]}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{ flex: 1, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                  >
                    {job.target.value}
                  </Typography>
                  <StateBadge state={job.state} />
                </Box>

                <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
                  {job.id.slice(0, 18)}…
                </Typography>

                {job.state === 'running' && (
                  job.repos_total > 0
                    ? <LinearProgress variant="determinate" value={Math.round(job.progress * 100)} sx={{ width: '100%', mt: 0.5, height: 2, borderRadius: 1 }} />
                    : <LinearProgress sx={{ width: '100%', mt: 0.5, height: 2, borderRadius: 1 }} />
                )}
              </ListItemButton>
            ))}
          </List>
        )}
      </Box>
    </Box>
  )
}
