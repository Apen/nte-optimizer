import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { applyLocalization } from '../i18n'
import type { LocalizationCatalog } from '../types'
import { ResourcesPage } from './inventory-pages'

const catalog: LocalizationCatalog = {
  locale: 'en',
  ui: {
    account_inventory: 'Account inventory',
    count_resources: 'Resources',
    imported_one: '{count} item imported from your account.',
    imported_many: '{count} items imported from your account.',
    search_collection: 'Search in {title}...',
  },
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}

describe('ResourcesPage', () => {
  beforeEach(() => applyLocalization('en', catalog))

  it('shows the resource name and quantity without the technical ID or redundant quantity label', () => {
    render(<ResourcesPage items={[{
      id: { solt: 1, serial: 2 },
      itemId: 'EquipmentUpMaterial_lv3',
      name: 'Manhole Boss',
      quantity: 1224,
    }]} />)

    expect(screen.getByText('Manhole Boss')).toBeInTheDocument()
    expect(screen.getByText('1,224')).toBeInTheDocument()
    expect(screen.queryByText('EquipmentUpMaterial_lv3')).not.toBeInTheDocument()
    expect(screen.queryByText('Owned quantity')).not.toBeInTheDocument()
  })
})
