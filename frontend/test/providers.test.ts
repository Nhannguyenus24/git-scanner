import { describe, expect, it } from 'vitest'
import { CATEGORY_ORDER, providerMeta } from '../src/providers'

describe('provider metadata', () => {
  it('keeps known providers in a supported category', () => {
    expect(providerMeta('github')).toEqual({
      label: 'GitHub',
      category: 'DevOps & Observability',
    })
    expect(CATEGORY_ORDER).toContain(providerMeta('github').category)
  })

  it('renders newly added provider slugs without a frontend change', () => {
    expect(providerMeta('new-provider')).toEqual({
      label: 'New Provider',
      category: 'Other',
    })
  })
})
