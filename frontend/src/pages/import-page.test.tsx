import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyLocalization } from '../i18n'
import type { AccountImportSummary, LocalizationCatalog } from '../types'
import { ImportPage } from './import-page'

const scanAndImport = vi.fn()
const recoverAndScanAccount = vi.fn()
const lastScanLog = vi.fn()

vi.mock('../../wailsjs/go/main/DesktopApp', () => ({
  ScanAndImportAccount: (...args: unknown[]) => scanAndImport(...args),
  RecoverAndScanAccount: (...args: unknown[]) => recoverAndScanAccount(...args),
  LastScanLog: () => lastScanLog(),
}))

const catalog: LocalizationCatalog = {
  locale: 'fr',
  ui: {
    scan_start: 'Lancer le scan',
    scan_running: 'Scan en cours',
    scan_waiting: 'En attente',
    scan_finished: '{characters} personnages, {pieces} équipements, {arcs} arcs',
    no_account_data: 'Aucune donnée',
    scan_log_title: 'Diagnostic du scan',
    scan_log_privacy: 'Local uniquement',
    scan_log_stage_pktmon_recovery: 'Récupération de Pktmon',
    scan_log_stage_pktmon_status: 'État de la capture Windows',
    scan_log_stage_validation: 'Validation',
    scan_log_status_incomplete: 'données requises manquantes',
    scan_log_status_requested: 'demandé par l’utilisateur',
    scan_log_status_skipped: 'ignoré après l’arrêt demandé de Pktmon',
    scan_log_count_equipment: 'équipement',
    scan_error_generic: 'Le scan n’a pas abouti. Consulte et copie le journal de diagnostic pour connaître la cause.',
    scan_error_busy: 'Un scan est déjà en cours.',
    scan_recovery_title: 'Une capture Pktmon semble peut-être bloquée',
    scan_recovery_warning: 'Ce bouton arrête toute collecte Pktmon en cours sur Windows, y compris celles démarrées par un autre outil, puis lance un nouveau scan guidé.',
    scan_recovery_button: 'Arrêter Pktmon et relancer le scan',
    scan_recovery_running: 'Arrêt de Pktmon et scan en cours…',
    scan_recovery_failed: 'La récupération Pktmon n’a pas terminé le scan. Consulte le journal de diagnostic et réessaie.',
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
    recoverAndScanAccount.mockReset()
    lastScanLog.mockReset()
    recoverAndScanAccount.mockResolvedValue(emptySummary)
    lastScanLog.mockResolvedValue([])
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

    expect(await screen.findByRole('alert')).toHaveTextContent('Le scan n’a pas abouti. Consulte et copie le journal de diagnostic pour connaître la cause.')
    expect(screen.getByRole('button', { name: 'Lancer le scan' })).toBeEnabled()
  })

  it('shows privacy-safe scan steps after a failed attempt', async () => {
    scanAndImport.mockRejectedValue(new Error('incomplete capture'))
    lastScanLog.mockResolvedValue([{ time: '2026-09-23T10:00:00Z', stage: 'validation', status: 'incomplete', counts: { equipment: 0 } }])
    render(<ImportPage summary={emptySummary} locale="fr" onImported={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Lancer le scan' }))
    expect(await screen.findByText('données requises manquantes')).toBeInTheDocument()
    expect(screen.getByText('équipement: 0')).toBeInTheDocument()
  })

  it('offers explicit Pktmon recovery after an uncertain capture status', async () => {
    const failedScan = [{ time: '2026-09-23T10:00:00Z', stage: 'capture', status: 'failed', code: 'pktmon_status_unknown' }]
    const recoveredScan = [
      { time: '2026-09-23T10:01:00Z', stage: 'pktmon_recovery', status: 'requested' },
      { time: '2026-09-23T10:01:01Z', stage: 'pktmon_status', status: 'skipped' },
      { time: '2026-09-23T10:01:02Z', stage: 'pktmon_recovery', status: 'complete' },
    ]
    lastScanLog.mockResolvedValueOnce(failedScan).mockResolvedValueOnce(recoveredScan)
    const imported: AccountImportSummary = { has_import: true, characters: 4, modules: 10, cartridges: 6, weapons: 3 }
    const onImported = vi.fn().mockResolvedValue(undefined)
    recoverAndScanAccount.mockResolvedValue(imported)
    render(<ImportPage summary={emptySummary} locale="fr" onImported={onImported} />)

    expect(await screen.findByText('Ce bouton arrête toute collecte Pktmon en cours sur Windows, y compris celles démarrées par un autre outil, puis lance un nouveau scan guidé.')).toBeInTheDocument()
    await userEvent.click(await screen.findByRole('button', { name: 'Arrêter Pktmon et relancer le scan' }))

    expect(recoverAndScanAccount).toHaveBeenCalledWith('fr', 35)
    expect(onImported).toHaveBeenCalledWith(imported)
    expect(await screen.findByRole('status')).toHaveTextContent('4 personnages, 16 équipements, 3 arcs')
    expect(screen.queryByRole('region', { name: 'Une capture Pktmon semble peut-être bloquée' })).not.toBeInTheDocument()
  })
})
