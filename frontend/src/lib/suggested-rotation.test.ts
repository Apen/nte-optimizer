import { describe, expect, it } from 'vitest'
import type { Result } from '../types'
import { suggestRotation } from './suggested-rotation'

function lineup(characterId: number, allies: number[], buff_effects?: Result['buff_effects']) {
  return {
    character: { characterId },
    team_allies: allies.map(character_id => ({ character_id })),
    buff_effects,
  } as Result
}

describe('suggestRotation', () => {
  it('places Discord and Daffodill before Iroi support and the Zankou combo', () => {
    const rotation = suggestRotation(lineup(1036, [1075, 1054, 1042], [
      { id: 'blackbird', character_id: 1042, trigger: 'ultimate', included: true },
      { id: 'iroi', character_id: 1075, trigger: 'redirect_press', included: true },
    ] as NonNullable<Result['buff_effects']>))

    expect(rotation?.missingReaction).toBeUndefined()
    expect(rotation?.steps.map(step => [step.id, step.characterID])).toEqual([
      ['open', 1036], ['scorch', 1054], ['nova', 1042], ['discord', 1054],
      ['break', 1054], ['buff:1075:redirect_press', 1075], ['burst', 1036],
    ])
  })

  it('selects Lacrimosa for Nova when both reference Chaos allies are present', () => {
    const rotation = suggestRotation(lineup(1042, [1054, 1020, 1004]))

    expect(rotation?.missingReaction).toBeUndefined()
    expect(rotation?.steps.find(step => step.id === 'nova')?.characterID).toBe(1004)
  })

  it('keeps Akane’s personal combo and adds Blackbird only when selected', () => {
    expect(suggestRotation(lineup(1057, []))?.steps.map(step => step.id)).toEqual(['burst'])
    expect(suggestRotation(lineup(1057, [1042]))?.steps.map(step => step.id)).toEqual(['blackbird', 'burst', 'malice'])
  })

  it('adds Hexed preparation only when Nanally has an Incantation ally', () => {
    expect(suggestRotation(lineup(1010, [1075]))?.missingReaction).toBe('rotation_reaction_hexed')
    expect(suggestRotation(lineup(1010, [1075, 1036]))?.steps.map(step => step.characterID)).toEqual([1036, 1010])
  })

  it('assigns the Blossom setup to Zero and the selected Anima ally', () => {
    expect(suggestRotation(lineup(1052, [1051, 1055, 1010]))?.steps.map(step => step.characterID))
      .toEqual([1052, 1051, 1055, 1010, 1052])
    expect(suggestRotation(lineup(1052, [1051]))?.steps.map(step => step.id)).toEqual(['record', 'burst'])
  })

  it.each([
    [1004, [1003, 1042], ['incantation', 'scorch', 'psyche', 'burst']],
    [1054, [1036, 1020], ['incantation', 'scorch', 'psyche', 'burst']],
    [1023, [1054], ['open', 'scorch', 'burst']],
    [1071, [1051], ['cosmos', 'burst']],
  ] as const)('keeps one combo for character %i with or without reaction allies', (id, allies, steps) => {
    expect(suggestRotation(lineup(id, [...allies]))?.steps.map(step => step.id)).toEqual(steps)
    expect(suggestRotation(lineup(id, []))?.steps.map(step => step.id)).toEqual(['burst'])
  })

  it('uses an eligible substitute for Charge and keeps the combo when Charge is unavailable', () => {
    const complete = suggestRotation(lineup(1076, [1075, 1051, 1008]))
    expect(complete?.steps.map(step => step.characterID)).toEqual([1076, 1008, 1051, 1075, 1076])

    const incomplete = suggestRotation(lineup(1076, [1075, 1051]))
    expect(incomplete?.missingReaction).toBe('rotation_reaction_charge')
    expect(incomplete?.steps.map(step => step.id)).toEqual(['stacks', 'burst'])
  })
})
