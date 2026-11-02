import { describe, expect, it } from 'vitest'
import fixture from '../../internal/testdata/contracts/bridge.json'
import type { AccountBuffPage, InventoryModule, ProfileSettings } from './types'

// The Go test checks that these same objects match actual JSON serialization.
const module: InventoryModule = fixture.module
const settings: ProfileSettings = fixture.settings
const bonuses: AccountBuffPage = fixture.bonuses

describe('shared bridge contract', () => {
  it('preserves inactive attributes, explicit strict floors and nullable bonuses', () => {
    expect(module.sub_stats_active).toBe(false)
    expect(settings.goals?.CritBase.minimum).toBe(.4)
    expect(settings.goals?.CritBase.custom).toBe(true)
    expect(bonuses.characters[0].bond_stats).toBeNull()
  })
})
