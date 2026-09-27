import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { applyLocalization } from '../i18n'
import { PresentationProvider } from '../presentation'
import type { LocalizationCatalog, PanelBonus, PotentialHousingEffect, StatSummary } from '../types'
import { CurrentGameStats } from './current-game-stats'

const catalog: LocalizationCatalog = {
  locale: 'en',
  ui: {
    current_stats: 'Current stats',
    current_stats_description: 'Final calculated stats',
    observed_panel_stats_description: 'Final values read from the in-game panel',
    possible_panel_bonuses: 'Possible additional in-game bonuses',
    possible_panel_bonuses_note: 'Positive differences between the scanned panel and the optimizer model. Their source is unknown; these values are diagnostic only and are not added to optimizer calculations.',
    panel_bonus_comparison: 'Panel: {panel} · Model: {calculated}',
    possible_housing_modifiers: 'Possible shared housing modifiers',
    possible_housing_modifiers_note: 'Panel-based hints only; not applied to optimizer calculations.',
    housing_modifier_evidence: 'Matched on {matching} of {observed} loaded characters',
    max_effects_value: 'Max effects: {value}',
    estimated_stats: 'Some stat sources are missing',
  },
  stats: {
    AtkBase: 'Base ATK', AtkFinal: 'ATK', AtkUp: 'ATK %', AtkAdd: 'Flat ATK',
    DefBase: 'Base DEF', DefFinal: 'DEF', DefUp: 'DEF %', DefAdd: 'Flat DEF', Endurance: 'Endurance',
    HPMaxBase: 'Base HP', HPFinal: 'HP', HPMaxUp: 'HP %', HPMaxAdd: 'Flat HP', HPUp: 'HP %',
    CritDamageBase: 'CRIT DMG', MagBase: 'Cycle Intensity', ElementalDMGBonus: 'Elemental DMG',
  },
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
}

const stats: StatSummary = {
  completeness: 'complete',
  missing_sources: [],
  sources: { base: { AtkBase: 1230 }, equipment: { AtkAdd: 468, HPMaxAdd: 6600 } },
  derived: {
    AtkBase: 1230, AtkUp: 0.038, AtkAdd: 468, AtkFinal: 1744,
    DefBase: 909, DefUp: 0.07, DefAdd: 24, DefFinal: 996,
    HPMaxBase: 15514, HPMaxUp: 0.088, HPMaxAdd: 6600, HPUp: 0.088, HPFinal: 23471,
    CritBase: 0.73,
  },
  derived_conditional: { AtkFinal: 2217, HPFinal: 23471 },
}

beforeEach(() => applyLocalization('en', catalog))

describe('CurrentGameStats', () => {
  it('shows final stats without base values, stat components, or conditional bonus values', () => {
    render(<PresentationProvider catalog={catalog}><CurrentGameStats calculatedStats={stats} /></PresentationProvider>)

    expect(screen.getByText('Current stats')).toBeInTheDocument()
    expect(screen.getByText('1,744')).toBeInTheDocument()
    expect(screen.getByText('996')).toBeInTheDocument()
    expect(screen.getByText('23,471')).toBeInTheDocument()
    expect(screen.getByText('73.0 %')).toBeInTheDocument()
    for (const componentLabel of ['Base ATK', 'ATK %', 'Flat ATK', 'Base DEF', 'DEF %', 'Flat DEF', 'Base HP', 'HP %', 'Flat HP']) {
      expect(screen.queryByText(componentLabel)).not.toBeInTheDocument()
    }
    expect(screen.queryByText('2,217')).not.toBeInTheDocument()
    expect(screen.queryByText(/Max effects/)).not.toBeInTheDocument()
    expect(screen.queryByText('1,230')).not.toBeInTheDocument()
    expect(screen.queryByText('468')).not.toBeInTheDocument()
  })

  it('prefers final values observed by the scanner over the calculated estimate', () => {
    render(<PresentationProvider catalog={catalog}><CurrentGameStats
      calculatedStats={stats}
      observedPanelStats={{
        maxHp: 23471.475,
        attack: 1748.125,
        defense: 1005,
        critRate: 0.73,
        critDamage: 2.304,
        chargeEfficiency: 1,
        cycleIntensity: 172,
        universalDamageBonus: 0.19999999,
        elementalDamageBonus: 0.1,
        panelBase: { maxHp: 15514, attack: 1230, defense: 909 },
        source: 'unreal_attribute_set_and_equipment',
      }}
    /></PresentationProvider>)

    expect(screen.getByText('Final values read from the in-game panel')).toBeInTheDocument()
    expect(screen.getByText('1,748')).toBeInTheDocument()
    expect(screen.getByText('1,005')).toBeInTheDocument()
    expect(screen.getByText('23,471')).toBeInTheDocument()
    expect(screen.getByText('230.4 %')).toBeInTheDocument()
    expect(screen.getByText('20.0 %')).toBeInTheDocument()
    expect(screen.getByText('10.0 %')).toBeInTheDocument()
    expect(screen.queryByText('1,744')).not.toBeInTheDocument()
    expect(screen.queryByText('996')).not.toBeInTheDocument()
    expect(screen.queryByText('228.0 %')).not.toBeInTheDocument()
    expect(screen.queryByText('Base ATK')).not.toBeInTheDocument()
    expect(screen.queryByText('1,230')).not.toBeInTheDocument()
    expect(screen.queryByText('Stats are estimated: some account bonuses or temporary effects are not available in imported data.')).not.toBeInTheDocument()
  })

  it('shows repeated panel residuals as diagnostic housing candidates', () => {
    const candidates: PotentialHousingEffect[] = [
      { modifier_id: 'yaodao_1', property_id: 'AtkAdd', panel_property: 'AtkFinal', value: 4, percent: false, matching_characters: 4, observed_characters: 4 },
      { modifier_id: 'quantao_5', property_id: 'CritDamageBase', panel_property: 'CritDamageBase', value: .024, percent: true, matching_characters: 4, observed_characters: 4 },
    ]
    render(<PresentationProvider catalog={catalog}><CurrentGameStats
      observedPanelStats={{ attack: 1748, critDamage: 2.304 }}
      calculatedStats={stats}
      potentialHousingEffects={candidates}
    /></PresentationProvider>)

    expect(screen.getByText('Possible shared housing modifiers')).toBeInTheDocument()
    expect(screen.getByText('Panel-based hints only; not applied to optimizer calculations.')).toBeInTheDocument()
    expect(screen.getAllByText('ATK')).toHaveLength(2)
    expect(screen.getByText('+4')).toBeInTheDocument()
    expect(screen.getByText('+2.4 %')).toBeInTheDocument()
    expect(screen.getAllByText('Matched on 4 of 4 loaded characters')).toHaveLength(2)
    expect(screen.queryByText('yaodao_1')).not.toBeInTheDocument()
  })

  it('shows positive panel residuals without requiring a source attribution', () => {
    const bonuses: PanelBonus[] = [
      { property_id: 'AtkFinal', observed_value: 1748.125, calculated_value: 1744, difference: 4 },
      { property_id: 'CritDamageBase', observed_value: 2.304, calculated_value: 2.28, difference: .024 },
      { property_id: 'MagBase', observed_value: 172, calculated_value: 72, difference: 100 },
    ]
    const housing: PotentialHousingEffect[] = [
      { modifier_id: 'yaodao_1', property_id: 'AtkAdd', panel_property: 'AtkFinal', value: 4, percent: false, matching_characters: 4, observed_characters: 4 },
    ]
    render(<PresentationProvider catalog={catalog}><CurrentGameStats
      observedPanelStats={{ attack: 1748.125, critDamage: 2.304, cycleIntensity: 172 }}
      calculatedStats={stats}
      possiblePanelBonuses={bonuses}
      potentialHousingEffects={housing}
    /></PresentationProvider>)

    expect(screen.getByText('Possible additional in-game bonuses')).toBeInTheDocument()
    expect(screen.getByText('Positive differences between the scanned panel and the optimizer model. Their source is unknown; these values are diagnostic only and are not added to optimizer calculations.')).toBeInTheDocument()
    expect(screen.getByText('+4')).toBeInTheDocument()
    expect(screen.getByText('+2.4 %')).toBeInTheDocument()
    expect(screen.getByText('+100')).toBeInTheDocument()
    expect(screen.getByText('Panel: 1,748 · Model: 1,744')).toBeInTheDocument()
    expect(screen.queryByText('Possible shared housing modifiers')).not.toBeInTheDocument()
  })
})
