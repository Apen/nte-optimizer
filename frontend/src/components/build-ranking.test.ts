import { describe, expect, it } from 'vitest'

import { buildResultColumns, objectiveFormula } from './build-ranking'

describe('objectiveFormula', () => {
  it('shows the resolved scale for Beta', () => {
    const explanation = objectiveFormula(140 / 360, 1, 360, 140, 120)
    expect(explanation).toContain('140')
    expect(explanation).toContain('120')
    expect(explanation).not.toContain('⁴')
  })

  it('preserves the legacy explanation for other goals', () => {
    expect(objectiveFormula(140 / 360, 1)).toContain('⁴')
  })
})

describe('buildResultColumns', () => {
  it('uses the visible goals and selected main stats without unrelated damage columns', () => {
    const columns = buildResultColumns([
      { property_id: 'AtkFinal' },
      { property_id: 'UnbalIntensityBase' },
      { property_id: 'DamageUpChaosBase' },
      { property_id: 'DamageUpGeneralBase' },
    ], ['DamageUpGeneralBase'], ['CritBase', 'DamageUpChaosBase'])
    expect(columns).toEqual(['BasicDamageIndex', 'AtkFinal', 'UnbalIntensityBase', 'DamageUpChaosBase', 'CritBase'])
    expect(columns).not.toContain('DamageUpIncantationBase')
  })
})
