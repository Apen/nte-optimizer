import { fireEvent, render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { beforeEach, describe, expect, it } from 'vitest'

import { applyLocalization } from '../i18n'
import type { Result } from '../types'
import { AdvancedResultDetails } from './advanced-result-details'

const result = {
  solution: { module_score: 1, cartridge_score: 2, set_bonus_score: 3, visited_states: 42, complete: true },
  stats: { completeness: 'complete', sources: {}, derived: {}, derived_conditional: {} },
  include_equipped: false,
  excluded_equipped: 2,
} as unknown as Result

beforeEach(() => applyLocalization('en', {
  locale: 'en',
  ui: { understand_result: 'Understand the result', module_preview: 'Equipment preview' },
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}))

describe('advanced result details dialog', () => {
  it('opens the result explanation from its button', () => {
    render(createElement(AdvancedResultDetails, { result }))

    fireEvent.click(screen.getByRole('button', { name: 'Understand the result' }))

    expect(screen.getByRole('dialog', { name: 'Understand the result' })).toBeInTheDocument()
  })
})
