import { fireEvent, render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { beforeEach, describe, expect, it } from 'vitest'

import { applyLocalization } from '../i18n'
import { PresentationProvider } from '../presentation'
import type { LocalizationCatalog, Result } from '../types'
import { AdvancedResultDetails } from './advanced-result-details'
import { ArcEffectDescription } from './equipment-cards'

const result = {
  solution: { module_score: 1, cartridge_score: 2, set_bonus_score: 3, set_matched_count: 4, visited_states: 42, complete: true },
  stats: { completeness: 'complete', sources: {
    set: { DamageUpIncantationBase: .1 },
    set_conditional: { AtkUp: .36 },
    weapon: { AtkBase: 570, CritRate: .24 },
    weapon_conditional: { CritDamageBase: .63 },
    weapon_permanent: { CritRate: .16 },
  }, derived: {}, derived_conditional: {} },
  include_equipped: false,
  excluded_equipped: 2,
  set: { id: 'Suit4', name: 'Crimson: Twin Butterflies', required_geometries: [] },
  weapon: { forkId: 'fork_DemonBlade', name: 'Ravenous Blade', level: 80, breakthrough: 6, star: 1 },
} as unknown as Result

const presentation: LocalizationCatalog = {
	locale: 'en', ui: {}, stats: {}, qualities: {}, geometries: {}, stat_sources: {
		set: 'Set bonus', set_conditional: 'Conditional set bonus',
		weapon: 'Arc', weapon_conditional: 'Conditional Arc effect', weapon_permanent: 'Permanent Arc effect',
	}, damage: {},
	set_effects: {
		Suit4_2: 'Epic (2): Incantation DMG +10%.',
		Suit4_4: 'Legendary (4): ATK +6% per stack, up to 6 stacks.',
	},
	fork_effects: {
		fork_DemonBlade_1: 'Increases Crit Rate by <lv>{0}</> and Crit DMG by <lv>{1}</> for <lv>{2}</>s.',
		fork_BlackBook_3: 'Break intensity <lv>{0}</>. The tome lasts <lv>{1}</>s, marks an enemy every <lv>{2}</>s, boosts Chaos DMG by <lv>{3}</> and deals <lv>{4}</> ATK as damage.',
		fork_mamen_2: 'Cosmos DMG increases by <lv>{1}</> for every <lv>{0}</> fons held.',
	},
	fork_effect_parameters: {
		fork_DemonBlade_1: [
			{ name_id: 'buff_DemonBlade_Crit', value: .16, is_percent: true },
			{ name_id: 'buff_DemonBlade_CritDamageUp', value: .09, is_percent: true },
			{ name_id: 'buff_DemonBlade_CD', value: 15, is_percent: false },
		],
		fork_BlackBook_3: [
			{ name_id: 'buff_BlackBook2_Unbal', value: 74, is_percent: false },
			{ name_id: 'buff_BlackBook2_CD', value: 20, is_percent: false },
			{ name_id: 'buff_BlackBook2_CD2', value: 5, is_percent: false },
			{ name_id: 'buff_BlackBook2_DamageUpChaosBase', value: .28, is_percent: true },
			{ name_id: 'buff_BlackBook2_SkillDamage', value: 2.6, is_percent: true },
		],
		fork_mamen_2: [
			{ name_id: 'buff_mamen_fons', value: 100000, is_percent: false },
			{ name_id: 'buff_mamen_CosmosUp', value: .03, is_percent: true },
		],
	},
}

beforeEach(() => applyLocalization('en', {
  locale: 'en',
  ui: { understand_result: 'Understand the result', module_preview: 'Equipment preview', arc_effect: 'Arc effect', effect_values_unresolved: 'Some effect values are unavailable in the extracted data.' },
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}))

describe('advanced result details dialog', () => {
  it('opens the result explanation from its button', () => {
    render(createElement(PresentationProvider, { catalog: presentation, children: createElement(AdvancedResultDetails, { result }) }))

    fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

    expect(screen.getByRole('dialog', { name: 'Understand the result' })).toBeInTheDocument()
  })

	it('merges projected Arc effects into the resolved datamine description', () => {
		render(createElement(PresentationProvider, { catalog: presentation, children: createElement(AdvancedResultDetails, { result }) }))
		fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

		expect(screen.getByText('Increases Crit Rate by 16% and Crit DMG by 9% for 15s.')).toBeInTheDocument()
		expect(screen.getByText('Arc effect')).toBeInTheDocument()
		expect(screen.queryByText('Conditional Arc effect')).not.toBeInTheDocument()
		expect(screen.queryByText('Permanent Arc effect')).not.toBeInTheDocument()
	})

	it('merges active set bonus rows into their datamine descriptions', () => {
		render(createElement(PresentationProvider, { catalog: presentation, children: createElement(AdvancedResultDetails, { result }) }))
		fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

		expect(screen.getByText('Crimson: Twin Butterflies')).toBeInTheDocument()
		expect(screen.getByText('Epic (2): Incantation DMG +10%.')).toBeInTheDocument()
		expect(screen.getByText('Legendary (4): ATK +6% per stack, up to 6 stacks.')).toBeInTheDocument()
		expect(screen.queryByText('Conditional set bonus')).not.toBeInTheDocument()
		expect(screen.queryByText('Set bonus')).not.toBeInTheDocument()
	})

	it('keeps projected Arc effect rows when parameterized description data is incomplete', () => {
		const incompletePresentation: LocalizationCatalog = { ...presentation, fork_effect_parameters: {} }
		render(createElement(PresentationProvider, { catalog: incompletePresentation, children: createElement(AdvancedResultDetails, { result }) }))
		fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

		expect(screen.getByText('Conditional Arc effect')).toBeInTheDocument()
		expect(screen.getByText('Permanent Arc effect')).toBeInTheDocument()
		expect(screen.getByText(/Some effect values are unavailable/)).toBeInTheDocument()
	})

	it('keeps projected set bonus rows when an active tier description is unavailable', () => {
		const incompletePresentation: LocalizationCatalog = { ...presentation, set_effects: { Suit4_2: 'Epic (2): Incantation DMG +10%.' } }
		render(createElement(PresentationProvider, { catalog: incompletePresentation, children: createElement(AdvancedResultDetails, { result }) }))
		fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

		expect(screen.getByText('Set bonus')).toBeInTheDocument()
		expect(screen.getByText('Conditional set bonus')).toBeInTheDocument()
	})

	it.each([
		{ forkID: 'fork_BlackBook', star: 3, expected: 'Break intensity 74. The tome lasts 20s, marks an enemy every 5s, boosts Chaos DMG by 28% and deals 260% ATK as damage.' },
		{ forkID: 'fork_mamen', star: 2, expected: 'Cosmos DMG increases by 3% for every 100,000 fons held.' },
	])('resolves $forkID at upgrade $star from its ordered game parameters', ({ forkID, star, expected }) => {
		render(createElement(PresentationProvider, { catalog: presentation, children: createElement(ArcEffectDescription, { forkID, star }) }))

		expect(screen.getByText(expected)).toBeInTheDocument()
	})
})
