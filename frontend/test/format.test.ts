import { describe, expect, it } from 'vitest'
import { commitLink, fileLink, monthLabel, paletteFor } from '../src/format'

describe('format helpers', () => {
  it('assigns stable chart colours and uses the fallback for overflow', () => {
    expect(paletteFor(['a', 'b', 'c'], ['red', 'blue'], 'gray')).toEqual({
      a: 'red',
      b: 'blue',
      c: 'gray',
    })
  })

  it('builds safe GitHub links only for GitHub repositories', () => {
    expect(fileLink('https://github.com/acme/app.git', 'src/main.ts', 12, 'abc123', false))
      .toBe('https://github.com/acme/app/blob/abc123/src/main.ts#L12')
    expect(commitLink('https://gitlab.com/acme/app', 'abc123')).toBeUndefined()
    expect(fileLink('https://github.com/acme/app', 'src/a b.ts', 0, '', true))
      .toBe('https://github.com/acme/app/blob/HEAD/src/a%20b.ts')
  })

  it('formats valid months and preserves invalid input', () => {
    expect(monthLabel('2026-03')).toBe("Mar '26")
    expect(monthLabel('not-a-month')).toBe('not-a-month')
  })
})
