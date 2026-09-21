import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyLocalization } from '../i18n'
import type { AccountImportSummary, LocalizationCatalog } from '../types'
import { ImportPage } from './import-page'

const scanAndImport = vi.fn()

vi.mock('../../wailsjs/go/main/DesktopApp', () => ({
  ScanAndImportAccount: (...args: unknown[]) => scanAndImport(...args),
}))

const catalog: LocalizationCatalog = {
  locale: 'fr',
  ui: {
    scan_start: 'Lancer le scan',
    scan_running: 'Scan en cours',
    scan_waiting: 'En attente',
    scan_finished: '{characters} personnages, {pieces} équipements, {arcs} arcs',
    no_account_data: 'Aucune donnée',
  },
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}

const emptySummary: AccountImportSummary = {
  has_import: false,
  characters: 0,
  modules: 0,
  cartridges: 0,
  weapons: 0,
}

describe('ImportPage', () => {
  beforeEach(() => {
    applyLocalization('fr', catalog)
    scanAndImport.mockReset()
  })

  it('runs the guided scan with the default duration and refreshes imported data', async () => {
    const imported: AccountImportSummary = { has_import: true, characters: 4, modules: 10, cartridges: 6, weapons: 3 }
    const onImported = vi.fn().mockResolvedValue(undefined)
    scanAndImport.mockResolvedValue(imported)
    render(<ImportPage summary={emptySummary} locale="fr" onImported={onImported} />)

    await userEvent.click(screen.getByRole('button', { name: 'Lancer le scan' }))

    expect(scanAndImport).toHaveBeenCalledWith('fr', 35)
    expect(onImported).toHaveBeenCalledWith(imported)
    expect(await screen.findByRole('status')).toHaveTextContent('4 personnages, 16 équipements, 3 arcs')
  })

  it('shows the scanner error and allows another attempt', async () => {
    scanAndImport.mockRejectedValue(new Error('UAC refusé'))
    render(<ImportPage summary={emptySummary} locale="fr" onImported={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Lancer le scan' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('UAC refusé')
    expect(screen.getByRole('button', { name: 'Lancer le scan' })).toBeEnabled()
  })
})
