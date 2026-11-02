import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import type { WorkspaceCharacter } from './types'

const api = vi.hoisted(() => ({ save: vi.fn(), target: vi.fn() }))
vi.mock('../wailsjs/go/main/DesktopApp', () => ({
  Profiles: async () => [1036, 1004].map(id => ({ id: `p${id}`, character_id: id, name: `P${id}`, main_stats: [], weights: { CritBase: 1 } })),
  BuildWorkspace: async () => ({ characters: [1036, 1004].map(id => ({ character_id: id, name: `C${id}`, level: 80, awaken_level: 0, priority: 1 })) }),
  EquipmentCatalog: async () => ({ modules: [], cartridges: [], arcs: [], resources: [] }),
  Localization: async () => ({ locale: 'en', ui: {}, stats: {}, qualities: {}, geometries: {}, stat_sources: {}, damage: {} }),
  AccountImportStatus: async () => ({ has_import: false }), CheckForUpdate: async () => ({ available: false }),
  Target: (...args: unknown[]) => api.target(...args), SaveProfileSettings: (...args: unknown[]) => api.save(...args),
  StopOptimization: async () => false,
}))
vi.mock('./components/app-navigation', () => ({
  AppSidebar: ({ onPage }: { onPage: (page: string) => void }) => <button onClick={() => onPage('characters')}>Characters</button>,
  MobileNavigation: () => null,
}))
vi.mock('./pages/characters-page', () => ({ CharactersPage: ({ characters, onBuild }: { characters: WorkspaceCharacter[]; onBuild: (id: number) => void }) => <>{characters.map(character => <button key={character.character_id} onClick={() => onBuild(character.character_id)}>Open {character.character_id}</button>)}</> }))
vi.mock('./components/optimizer-configuration', async importOriginal => {
  const original = await importOriginal<typeof import('./components/optimizer-configuration')>()
  return { ...original, StatsEditor: ({ goals, onGoal }: { goals: Record<string, number>; onGoal: (id: string, value: number) => void }) => <input aria-label="Goal" value={goals.CritBase ?? 0} onChange={event => onGoal('CritBase', Number(event.target.value))} /> }
})

beforeEach(() => {
  api.save.mockReset().mockResolvedValue({})
  api.target.mockReset().mockResolvedValue({ name: 'Target', goals: [{ property_id: 'CritBase', minimum: .5, percent: true }] })
  Object.defineProperty(window, 'runtime', { configurable: true, value: { EventsOn: () => () => {} } })
})
afterEach(() => vi.useRealTimers())

describe('profile draft lifecycle', () => {
  it('persists A and B edits without a late A acknowledgment resetting B', async () => {
    let acknowledgeA!: () => void
    api.save.mockImplementationOnce(() => new Promise<void>(resolve => { acknowledgeA = resolve }))
    render(<App />)
    fireEvent.click(await screen.findByText('Open 1036'))
    await screen.findByLabelText('Goal')
    await act(async () => {})
    vi.useFakeTimers()
    fireEvent.change(screen.getByLabelText('Goal'), { target: { value: '61' } })
    fireEvent.click(screen.getByText('Characters'))
    fireEvent.click(screen.getByText('Open 1004'))
    await act(async () => {})
    fireEvent.change(screen.getByLabelText('Goal'), { target: { value: '72' } })
    await act(async () => { acknowledgeA(); await vi.advanceTimersByTimeAsync(450) })
    expect(screen.getByLabelText('Goal')).toHaveValue('72')
    expect(api.save.mock.calls.map(([id, settings]) => [id, settings.goals.CritBase.target])).toEqual([['p1036', .61], ['p1004', .72]])
  })
})
