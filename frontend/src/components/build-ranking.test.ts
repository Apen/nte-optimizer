import { describe, expect, it } from 'vitest'

import { buildResultColumns, buildResultSignature, limitBuildResults, objectiveFormula } from './build-ranking'

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

describe('build result list', () => {
  it('keeps distinct equipment combinations and caps the displayed list at 100', () => {
    const builds = Array.from({ length: 120 }, (_, index) => index)
    expect(limitBuildResults(builds)).toEqual(builds.slice(0, 100))
  })

  it('identifies a build by its equipped modules, cartridge, Arc, and set', () => {
    const base = {
      modules: [{ module: { local_id: 'module-b' } }, { module: { local_id: 'module-a' } }],
      cartridge: { local_id: 'cartridge-1' },
      set: { id: 'set-1' },
      solution: { selected_weapon_id: 'arc-1', selected_cartridge_id: 'cartridge-1', selected_set_id: 'set-1' },
    }
    const sameItemsDifferentOrder = { ...base, modules: [...base.modules].reverse() }
    const differentBuild = { ...base, modules: [{ module: { local_id: 'module-c' } }] }

    expect(buildResultSignature(base)).toBe(buildResultSignature(sameItemsDifferentOrder))
    expect(buildResultSignature(base)).not.toBe(buildResultSignature(differentBuild))
  })
})
