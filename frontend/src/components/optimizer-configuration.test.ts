import { describe, expect, it } from 'vitest'

import { weightForGoal } from './optimizer-configuration'

describe('weightForGoal', () => {
  it('uses the strongest member of a derived-stat family', () => {
    expect(weightForGoal('AtkFinal', { weights: { AtkBase: 0.2, AtkUp: 0.8, AtkAdd: 0.4 } })).toBe(0.8)
  })

  it('never returns a negative weight', () => {
    expect(weightForGoal('CritBase', { weights: { CritBase: -1 } })).toBe(0)
  })
})
