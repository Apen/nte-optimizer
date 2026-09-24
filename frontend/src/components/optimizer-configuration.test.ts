import { fireEvent, render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyLocalization } from '../i18n'
import type { LocalizationCatalog, TargetPreset } from '../types'
import { StatsEditor, weightForGoal } from './optimizer-configuration'

const zeroWeightHint = 'Not included in the score'
const catalog: LocalizationCatalog = {
  locale: 'en',
  ui: {
    zero_weight_goal_hint: zeroWeightHint,
    weight_for: 'Weight for {name}',
    reference: 'Reference: {value}',
    remove_goal: 'Remove {name}',
    stats_section: 'Stats',
    reset: 'Reset',
    stats_main: 'Main stats',
    stats_objectives: 'Objectives',
    add_objective_stat: 'Add an objective stat…',
    statistic: 'Statistic',
    weight: 'Weight',
    objective_label: 'Objective',
    min_strict: 'Min',
    max_strict: 'Max',
    weight_help: 'Weight help',
    advanced_constraints: 'Advanced constraints',
    active_constraints_count: '{count} active',
    strict_limits_description: 'Strict minimums and maximums',
  },
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}

const preset: TargetPreset = {
  name: 'Test profile',
  goals: [{ property_id: 'CritBase', label: 'Critical rate', minimum: 0.2, percent: true }],
}

function renderStatsEditor(weight: number, disabledGoals: string[] = [], onAddGoal = vi.fn(), availableGoals = preset.goals) {
  return render(createElement(StatsEditor, {
    goalDefinitions: preset.goals,
    availableGoals,
    disabledGoals,
    strictGoals: [],
    strictMinimums: {},
    goals: { CritBase: 20 },
    maximums: {},
    weights: { main_stats: [], weights: { CritBase: weight } },
    availableMain: [],
    onGoal: vi.fn(),
    onMaximum: vi.fn(),
    onMinimum: vi.fn(),
    onWeight: vi.fn(),
    onMainStats: vi.fn(),
    onRemove: vi.fn(),
    onAddGoal,
    onReset: vi.fn().mockResolvedValue(undefined),
    onSave: vi.fn().mockResolvedValue(undefined),
  }))
}

beforeEach(() => applyLocalization('en', catalog))

describe('weightForGoal', () => {
  it('uses the strongest member of a derived-stat family', () => {
    expect(weightForGoal('AtkFinal', { weights: { AtkBase: 0.2, AtkUp: 0.8, AtkAdd: 0.4 } })).toBe(0.8)
  })

  it('never returns a negative weight', () => {
    expect(weightForGoal('CritBase', { weights: { CritBase: -1 } })).toBe(0)
  })
})

describe('zero-weight goal hint', () => {
  it('shows the localized explanation inline when weight is zero', () => {
    renderStatsEditor(0)

    expect(screen.getByRole('spinbutton', { name: 'Weight for Critical rate' })).toBeInTheDocument()
    expect(screen.getByText(zeroWeightHint)).toBeInTheDocument()
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('does not show the hint when the stat has a non-zero weight', () => {
    renderStatsEditor(0.5)

    expect(screen.queryByText(zeroWeightHint)).not.toBeInTheDocument()
  })

  it('exposes strict limits through an accessible disclosure control', () => {
    renderStatsEditor(0)

    const disclosure = screen.getByRole('button', { name: /Advanced constraints/ })
    expect(disclosure).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByText('Strict minimums and maximums')).toBeInTheDocument()

    fireEvent.click(disclosure)

    expect(disclosure).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('spinbutton', { name: 'Min Critical rate' })).toBeInTheDocument()
    expect(screen.getByRole('spinbutton', { name: 'Max Critical rate' })).toBeInTheDocument()
  })
})

describe('adding objective stats', () => {
  it('offers removed profile goals and lets the parent restore them individually', () => {
    const onAddGoal = vi.fn()
    renderStatsEditor(0, ['CritBase'], onAddGoal)

    expect(screen.queryByRole('spinbutton', { name: 'Weight for Critical rate' })).not.toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Add an objective stat…' }), { target: { value: 'CritBase' } })

    expect(onAddGoal).toHaveBeenCalledWith(preset.goals[0])
  })

  it('offers any localized stat that is not currently displayed', () => {
    const onAddGoal = vi.fn()
    const defense = { property_id: 'DefFinal', label: 'DEF', minimum: 0, percent: false }
    const { container } = renderStatsEditor(0, [], onAddGoal, [...preset.goals, defense])

    const addSelect = screen.getByRole('combobox', { name: 'Add an objective stat…' })
    expect(screen.getByRole('option', { name: 'DEF' })).toBeInTheDocument()
    const table = container.querySelector('.goal-matrix')!
    const toolbar = addSelect.closest('.goal-add-toolbar')!
    expect(table.compareDocumentPosition(toolbar) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    fireEvent.change(addSelect, { target: { value: 'DefFinal' } })

    expect(onAddGoal).toHaveBeenCalledWith(defense)
  })
})
