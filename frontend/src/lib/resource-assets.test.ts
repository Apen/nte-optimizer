import { describe, expect, it } from 'vitest'

import { resourceAssetID } from './resource-assets'

describe('resourceAssetID', () => {
  it('uses the generated canonical image name for case-only resource aliases', () => {
    expect(resourceAssetID('gold', { gold: 'Gold' })).toBe('Gold')
  })

  it('keeps the existing embedded multiplication-sign normalization', () => {
    expect(resourceAssetID('Currency×10')).toBe('Currencyx10')
  })
})
