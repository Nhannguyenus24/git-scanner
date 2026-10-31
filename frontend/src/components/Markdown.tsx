import React from 'react'
import Box from '@mui/material/Box'
import Link from '@mui/material/Link'
import Typography from '@mui/material/Typography'

// A small Markdown renderer for the bundled usage guide. The content is authored
// in this repository and trusted, so it builds React nodes directly (no
// dangerouslySetInnerHTML, no HTML passthrough) and supports just the subset the
// guide uses: #/##/### headings, paragraphs, - lists, ``` fences, `code`,
// **bold**, *italic*, [links](url) and --- rules.

export default function Markdown({ source }: { source: string }) {
  return <Box sx={{ '& > :first-of-type': { mt: 0 } }}>{renderBlocks(source)}</Box>
}

function renderBlocks(src: string): React.ReactNode[] {
  const lines = src.replace(/\r\n/g, '\n').split('\n')
  const out: React.ReactNode[] = []
  let i = 0
  let key = 0
  const push = (n: React.ReactNode) => out.push(<React.Fragment key={key++}>{n}</React.Fragment>)

  while (i < lines.length) {
    const line = lines[i]

    if (line.trim() === '') { i++; continue }

    // Fenced code block.
    if (line.startsWith('```')) {
      const body: string[] = []
      i++
      while (i < lines.length && !lines[i].startsWith('```')) { body.push(lines[i]); i++ }
      i++ // closing fence
      push(
        <Box component="pre" sx={{
          fontFamily: '"IBM Plex Mono", monospace', fontSize: 12.5, m: '0 0 16px', p: 1.5,
          bgcolor: 'action.hover', border: t => `1px solid ${t.palette.divider}`, borderRadius: 1,
          overflowX: 'auto', whiteSpace: 'pre',
        }}>{body.join('\n')}</Box>,
      )
      continue
    }

    // Horizontal rule.
    if (/^---+$/.test(line.trim())) {
      push(<Box component="hr" sx={{ border: 0, borderTop: t => `1px solid ${t.palette.divider}`, my: 3 }} />)
      i++
      continue
    }

    // Heading.
    const h = /^(#{1,3})\s+(.*)$/.exec(line)
    if (h) {
      const level = h[1].length
      const sx = level === 1
        ? { fontSize: 24, fontWeight: 800, mt: 0, mb: 1.5, pb: 1, borderBottom: (t: any) => `2px solid ${t.palette.divider}` }
        : level === 2
          ? { fontSize: 18, fontWeight: 700, mt: 3.5, mb: 1 }
          : { fontSize: 15, fontWeight: 700, mt: 2.5, mb: .75 }
      push(<Typography component={`h${level}` as 'h1'} sx={{ fontFamily: '"IBM Plex Mono", monospace', ...sx }}>{inline(h[2])}</Typography>)
      i++
      continue
    }

    // List (- or * bullets, or 1. ordered).
    const isUl = /^\s*[-*]\s+/.test(line)
    const isOl = /^\s*\d+\.\s+/.test(line)
    if (isUl || isOl) {
      const items: string[] = []
      const re = isUl ? /^\s*[-*]\s+/ : /^\s*\d+\.\s+/
      while (i < lines.length && re.test(lines[i])) {
        // Join a wrapped continuation line into the current item.
        let text = lines[i].replace(re, '')
        i++
        while (i < lines.length && lines[i].trim() !== '' && !re.test(lines[i]) && !/^(#{1,3})\s/.test(lines[i]) && !lines[i].startsWith('```')) {
          text += ' ' + lines[i].trim()
          i++
        }
        items.push(text)
      }
      push(
        <Box component={isOl ? 'ol' : 'ul'} sx={{ pl: 3, m: '0 0 16px', '& li': { mb: .5, fontSize: 14, lineHeight: 1.6 } }}>
          {items.map((t, k) => <li key={k}>{inline(t)}</li>)}
        </Box>,
      )
      continue
    }

    // Paragraph: gather until a blank line.
    const para: string[] = [line]
    i++
    while (i < lines.length && lines[i].trim() !== '' && !/^(#{1,3})\s/.test(lines[i]) && !lines[i].startsWith('```') && !/^\s*[-*]\s+/.test(lines[i]) && !/^\s*\d+\.\s+/.test(lines[i])) {
      para.push(lines[i]); i++
    }
    push(<Typography sx={{ fontSize: 14, lineHeight: 1.7, mb: 2, color: 'text.primary' }}>{inline(para.join(' '))}</Typography>)
  }

  return out
}

// inline renders `code`, **bold**, *italic* and [text](url) within a run of text.
// Italic needs a non-space after the opening star, so a bare "*" in prose stays
// literal.
function inline(text: string): React.ReactNode[] {
  const tokens = text.split(/(`[^`]+`|\*\*[^*]+\*\*|\*[^*\s][^*]*\*|\[[^\]]+\]\([^)]+\))/g)
  return tokens.filter(Boolean).map((tok, k) => {
    if (tok.startsWith('`') && tok.endsWith('`')) {
      return <Box component="code" key={k} sx={{
        fontFamily: '"IBM Plex Mono", monospace', fontSize: '.88em', px: .5, py: '1px',
        bgcolor: 'action.hover', border: t => `1px solid ${t.palette.divider}`, borderRadius: .5,
      }}>{tok.slice(1, -1)}</Box>
    }
    if (tok.startsWith('**') && tok.endsWith('**')) {
      return <Box component="strong" key={k} sx={{ fontWeight: 700 }}>{tok.slice(2, -2)}</Box>
    }
    if (tok.length > 2 && tok.startsWith('*') && tok.endsWith('*')) {
      return <Box component="em" key={k}>{tok.slice(1, -1)}</Box>
    }
    const link = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(tok)
    if (link) {
      return <Link key={k} href={link[2]} target="_blank" rel="noreferrer" underline="hover">{link[1]}</Link>
    }
    return <React.Fragment key={k}>{tok}</React.Fragment>
  })
}
