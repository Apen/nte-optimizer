import { expect, it } from 'vitest'
import { planBurstSetup } from './burst-setup'
import type { TeamPreview } from '../types'

it('reuses an existing ultimate for a shared set and schedules the shortest buffs last', () => {
  const effects: TeamPreview['conditional'] = [
    { id: 'set', kind: 'set', character_id: 2, detail: '', trigger: 'ultimate', source_ids: [2, 1], duration_seconds: 20, included: true },
    { id: 'short', kind: 'arc', character_id: 3, detail: '', trigger: 'ultimate', duration_seconds: 15, included: true },
    { id: 'long', kind: 'skill', character_id: 1, detail: '', trigger: 'ultimate', duration_seconds: 20, included: true },
    { id: 'heal', kind: 'arc', character_id: 2, detail: '', trigger: 'healing', rotation_action: 'redirect_press', duration_seconds: 20, included: true },
    { id: 'not-modeled', kind: 'skill', character_id: 2, detail: '', trigger: 'ultimate' },
  ]
  expect(planBurstSetup(effects).map(step => [step.characterID, step.action, step.effects.map(effect => effect.id)])).toEqual([
    [1, 'ultimate', ['long', 'set']],
    [2, 'redirect_press', ['heal']],
    [3, 'ultimate', ['short']],
  ])
})
