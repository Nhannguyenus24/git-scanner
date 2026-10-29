import Box from '@mui/material/Box'

/** A key cap, for printing a shortcut next to the thing it triggers. */
export default function ShortcutHint({ keys }: { keys: string }) {
  return (
    <Box component="kbd" sx={{
      fontFamily: '"IBM Plex Mono", ui-monospace, monospace', fontSize: 10.5, lineHeight: 1.6,
      px: .6, py: .1, borderRadius: .5, whiteSpace: 'nowrap',
      border: 1, borderColor: 'divider', bgcolor: 'action.hover', color: 'text.secondary',
    }}>{keys}</Box>
  )
}
