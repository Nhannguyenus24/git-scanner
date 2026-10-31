// @vitest-environment jsdom
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import Markdown from '../src/components/Markdown'
import { LANGUAGES } from '../src/i18n'

afterEach(cleanup)

const guide = (lang: string) => readFileSync(resolve(__dirname, `../src/docs/guide.${lang}.md`), 'utf8')
const headings = (src: string) => src.split('\n').filter(l => /^#{1,3}\s/.test(l)).map(l => l.split(' ')[0])

describe('usage guide', () => {
  // Translations drift silently: a section added to the English guide and
  // forgotten elsewhere still renders. Comparing the heading outline catches it.
  it('has the same section outline in every language', () => {
    const outline = headings(guide('en'))
    for (const { code } of LANGUAGES) expect(headings(guide(code)), code).toEqual(outline)
  })

  it('renders italic, bold and code inline', () => {
    const { container } = render(<Markdown source={'A *soft* and **hard** `x` * star'} />)
    expect(container.querySelector('em')?.textContent).toBe('soft')
    expect(container.querySelector('strong')?.textContent).toBe('hard')
    expect(container.querySelector('code')?.textContent).toBe('x')
    // A lone star in prose is not an italic marker.
    expect(container.textContent).toContain('* star')
  })

  it('leaves no unrendered markers in the guide', () => {
    for (const { code } of LANGUAGES) {
      const { container } = render(<Markdown source={guide(code)} />)
      expect(container.textContent, code).not.toMatch(/\*\*|`/)
      cleanup()
    }
  })
})
