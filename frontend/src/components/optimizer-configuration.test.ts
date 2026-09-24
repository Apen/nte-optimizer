import { render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyLocalization } from '../i18n'
import type { LocalizationCatalog, TargetPreset } from '../types'
import { StatsEditor, weightForGoal } from './optimizer-configuration'

const zeroWeightHint = 'Target does not affect ranking; use a weight or strict minimum.'
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
    statistic: 'Statistic',
    weight: 'Weight',
    objective_label: 'Objective',
    min_strict: 'Min',
    max_strict: 'Max',
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
  goals: [{ property_id: 'CritBase', label: 'Critical rate', minimum: 20, percent: true }],
}

function renderStatsEditor(weight: number) {
  return render(createElement(StatsEditor, {
    preset,
    disabledGoals: [],
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

describe('zero-weight goal warning', () => {
  it('shows the localized explanation in an accessible warning tooltip when weight is zero', () => {
    renderStatsEditor(0)

    const warning = screen.getByRole('button', { name: 'Weight for Critical rate' })
    const tooltip = screen.getByRole('tooltip')
    expect(screen.getByRole('spinbutton', { name: 'Weight for Critical rate' })).toBeInTheDocument()
    expect(warning).toHaveAttribute('aria-describedby', tooltip.id)
    expect(tooltip).toHaveTextContent(zeroWeightHint)
    expect(tooltip.className).toContain('group-hover:opacity-100')
    expect(tooltip.className).toContain('group-focus-within:opacity-100')
  })

  it('does not show a warning tooltip when the stat has a non-zero weight', () => {
    renderStatsEditor(0.5)

    expect(screen.queryByRole('button', { name: 'Weight for Critical rate' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })
})
