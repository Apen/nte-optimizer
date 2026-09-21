import { describe, expect, it } from 'vitest'
import { initialLocale } from './i18n'

describe('initialLocale', () => {
  it('defaults to English when no preference has been saved', () => {
    expect(initialLocale(null)).toBe('en')
  })

  it('keeps a saved supported locale', () => {
    expect(initialLocale('en')).toBe('en')
    expect(initialLocale('fr')).toBe('fr')
  })

  it('ignores an unsupported saved value', () => {
    expect(initialLocale('de')).toBe('en')
  })
})
