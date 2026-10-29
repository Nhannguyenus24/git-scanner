import Box from '@mui/material/Box'
import { useTheme } from '@mui/material/styles'
import safe from '../assets/undraw/safe.svg?raw'
import searching from '../assets/undraw/searching.svg?raw'
import folderFiles from '../assets/undraw/folder-files.svg?raw'
import theSearch from '../assets/undraw/the-search.svg?raw'
import emptyMailbox from '../assets/undraw/empty-mailbox.svg?raw'
import serverDown from '../assets/undraw/server-down.svg?raw'
import welcome from '../assets/undraw/welcome.svg?raw'
import noData from '../assets/undraw/no-data.svg?raw'
import celebration from '../assets/undraw/celebration.svg?raw'
import securityOn from '../assets/undraw/security-on.svg?raw'
import setup from '../assets/undraw/setup.svg?raw'
import bugDetected from '../assets/undraw/bug-detected.svg?raw'

// unDraw illustrations, inlined and re-inked from the active theme.
//
// They are inlined rather than used as <img src>: an external SVG is its own
// document and would not inherit the custom properties below, so it would stay
// unDraw violet on every preset. The files are build-time assets that went
// through scripts/process-undraw.py, which refuses any file containing a script
// element — nothing here comes from the API or from a finding.

export type IllustrationName =
  | 'safe' | 'searching' | 'repositories' | 'investigate' | 'mailbox' | 'offline'
  | 'welcome' | 'no-data' | 'celebration' | 'security' | 'setup' | 'detected'

const ART: Record<IllustrationName, string> = {
  'safe':         safe,
  'searching':    searching,
  'repositories': folderFiles,
  'investigate':  theSearch,
  'mailbox':      emptyMailbox,
  'offline':      serverDown,
  'welcome':      welcome,
  'no-data':      noData,
  'celebration':  celebration,
  'security':     securityOn,
  'setup':        setup,
  'detected':     bugDetected,
}

interface Props {
  name:   IllustrationName
  /** Max width in px. The art scales to fit and never grows past the column. */
  size?:  number
  /** Describes the picture for assistive tech; omit when the surrounding text
   *  already says it, which is the usual case for an empty state. */
  alt?:   string
}

export default function Illustration({ name, size = 220, alt }: Props) {
  const ill = useTheme().illustration
  return (
    <Box
      aria-hidden={alt ? undefined : true}
      role={alt ? 'img' : undefined}
      aria-label={alt}
      sx={{
        width: '100%', maxWidth: size, mx: 'auto',
        '--ud-accent':  ill.accent,
        '--ud-accent2': ill.accent2,
        '--ud-ink':     ill.ink,
        '--ud-ink2':    ill.ink2,
        '--ud-mid':     ill.mid,
        '--ud-soft':    ill.soft,
        '--ud-paper':   ill.paper,
        '& svg': { width: '100%', height: 'auto', display: 'block' },
      }}
      dangerouslySetInnerHTML={{ __html: ART[name] }}
    />
  )
}
