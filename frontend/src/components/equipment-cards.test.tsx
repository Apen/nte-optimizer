import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ModulePieceCard } from './equipment-cards'

describe('ModulePieceCard activation', () => {
  it('keeps inactive imported substats visible and identifies their status', () => {
    const item = {
      geometry: 'H_2',
      main_stats: [{ property_id: 'AtkAdd', value: 12, percent: false }],
      sub_stats: [{ property_id: 'CritBase', value: 0.03, percent: true }],
      sub_stats_active: false,
    }
    const { rerender } = render(<ModulePieceCard item={item} title="test module" />)
    expect(screen.getByText('secondary_attributes_inactive')).toBeTruthy()
    expect(screen.getByText('CritBase')).toBeTruthy()
    expect(screen.getByText('3.0 %')).toBeTruthy()
    expect(screen.getByText('AtkAdd')).toBeTruthy()

    rerender(<ModulePieceCard item={{ ...item, sub_stats_active: true }} title="test module" />)
    expect(screen.queryByText('secondary_attributes_inactive')).toBeNull()
    expect(screen.getByText('secondary_attributes')).toBeTruthy()
  })
})
