import { describe, expect, it } from 'vitest'
import type { DamageGroup } from '../types'
import { averageDamageGain } from './damage-gain'

const action: DamageGroup = {
  id: 'action', name: '', description: '', category: 'direct',
  build_damage: 9500, current_damage: 10000, build_non_crit: 0, build_crit: 0, instances: 10,
}

describe('averageDamageGain', () => {
  it('weights a direct loss and a reaction gain equally and uses the displayed reaction tick', () => {
    const scorch = {
      ...action, id: 'scorch', category: 'reaction', build_damage: 100, current_damage: 100,
      build_max_tick: 330, current_max_tick: 300,
      buffed: { build: 1000, current: 300, build_crit: 0, current_crit: 0 },
    }
    expect(averageDamageGain([action, scorch])).toBeCloseTo(0.025)
  })

  it('skips actions without a positive equipped baseline', () => {
    const missing = { ...action, current_damage: undefined }
    const zero = { ...action, current_damage: 0 }
    expect(averageDamageGain([missing, action, zero])).toBeCloseTo(-0.05)
    expect(averageDamageGain([missing, zero])).toBeUndefined()
    expect(averageDamageGain()).toBeUndefined()
  })

  it('excludes unresolved exclusive scenarios and counts a selected scenario once', () => {
    const condition = { exclusive_group: 'sub_ultimate', buff_id: 'enhanced' }
    const base = { ...action, id: 'base', build_damage: 11000, action_condition: { ...condition, buff_active: false } }
    const enhanced = { ...base, id: 'enhanced', build_damage: 12000, action_condition: { ...condition, buff_active: true } }
    expect(averageDamageGain([action, base, enhanced])).toBeCloseTo(-0.05)
    expect(averageDamageGain([base, enhanced])).toBeUndefined()
    expect(averageDamageGain([action, enhanced])).toBeCloseTo(0.075)
  })
})
