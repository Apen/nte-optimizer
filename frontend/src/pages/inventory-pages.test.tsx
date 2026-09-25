import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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
  beforeEach(() => {
    applyLocalization('en', catalog)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ aliases: { gold: 'Gold' } }) }))
  })
  afterEach(() => vi.unstubAllGlobals())

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

  it('renders every supplied resource regardless of its item family', () => {
    const items = Array.from({ length: 131 }, (_, index) => ({
      id: { solt: index + 1, serial: index + 1001 },
      itemId: `SyntheticResourceType_${index + 1}`,
      name: `Synthetic resource ${index + 1}`,
      quantity: index + 1,
    }))
    render(<ResourcesPage items={items} />)

    expect(screen.getByText('131 items imported from your account.')).toBeInTheDocument()
    expect(screen.getByText('Synthetic resource 1')).toBeInTheDocument()
    expect(screen.getByText('Synthetic resource 131')).toBeInTheDocument()
    expect(screen.getAllByRole('img')).toHaveLength(131)
  })

  it('uses the generated alias catalog to find resource icons', async () => {
    render(<ResourcesPage items={[{
      id: { solt: 1, serial: 2 },
      itemId: 'gold',
      name: 'Beetle Coin',
      quantity: 1,
    }]} />)

    await waitFor(() => {
      expect(screen.getByRole('img', { name: 'Beetle Coin' })).toHaveAttribute('src', '/game_ui/resources/Gold.png')
    })
  })
})
