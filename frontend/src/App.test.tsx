import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import type { TeamAlly, WorkspaceCharacter } from './types'

const api = vi.hoisted(() => ({
  save: vi.fn(), target: vi.fn(), preview: vi.fn(), optimize: vi.fn(), teams: new Map<string, TeamAlly[]>(),
}))
vi.mock('../wailsjs/go/main/DesktopApp', () => ({
  Profiles: async () => [1036, 1004].map(id => ({ id: `p${id}`, character_id: id, name: `P${id}`, main_stats: [], weights: { CritBase: 1 }, team_allies: api.teams.get(`p${id}`) })),
  BuildWorkspace: async () => ({ characters: [1036, 1004].map(id => ({ character_id: id, name: `C${id}`, level: 80, awaken_level: 0, priority: 1 })) }),
  EquipmentCatalog: async () => ({ modules: [], cartridges: [], arcs: [], resources: [] }),
  Localization: async () => ({ locale: 'en', ui: {
    team_ally: 'Ally {number}', find_best_build: 'Find best build', team_trigger_redirect_skill_or_enter_combat: 'After a Redirect Skill or on entering combat',
  }, stats: {}, qualities: {}, geometries: {}, stat_sources: {}, damage: {} }),
  AccountImportStatus: async () => ({ has_import: false }), CheckForUpdate: async () => ({ available: false }),
  Target: (...args: unknown[]) => api.target(...args), SaveProfileSettings: (...args: unknown[]) => api.save(...args),
  StopOptimization: async () => false,
  PreviewTeam: (...args: unknown[]) => api.preview(...args),
  OptimizeFlexibleSelection: (...args: unknown[]) => api.optimize(...args),
  LastOptimizationLog: async () => undefined,
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
  api.teams.clear()
  api.save.mockReset().mockResolvedValue({})
  api.target.mockReset().mockResolvedValue({ name: 'Target', goals: [{ property_id: 'CritBase', minimum: .5, percent: true }] })
  api.preview.mockReset().mockResolvedValue({ applied: [], conditional: [] })
  api.optimize.mockReset().mockResolvedValue(undefined)
  Object.defineProperty(window, 'runtime', { configurable: true, value: { EventsOn: () => () => {} } })
})

it('sends an Arc condition only when the user confirms it', async () => {
  const trigger = 'redirect_skill_or_enter_combat'
  api.teams.set('p1036', [{ character_id: 1004 }])
  api.preview.mockResolvedValue({
    applied: [], conditional: [],
    arc_conditions: [{ character_id: 1004, fork_id: 'fork_elyms', arc_name: 'Echoes on Hold', trigger, max_stacks: 1 }],
  })
  render(<App />)
  fireEvent.click(await screen.findByText('Open 1036'))
  const condition = await screen.findByLabelText(/Echoes on Hold.*Redirect Skill/)
  expect(condition).not.toBeChecked()
  const search = screen.getByRole('button', { name: 'Find best build' })
  await act(async () => fireEvent.click(search))
  expect(api.optimize.mock.calls[0][7]).not.toHaveProperty('arc_conditions')
  fireEvent.click(condition)
  await act(async () => fireEvent.click(search))
  expect(api.optimize.mock.calls[1][7].arc_conditions).toEqual([
    { character_id: 1004, fork_id: 'fork_elyms', trigger, stacks: 1 },
  ])
})
afterEach(() => vi.useRealTimers())

describe('profile draft lifecycle', () => {
  it('keeps an ally when reopening the same profile and restores it after relaunch', async () => {
    api.save.mockImplementation(async (id, settings) => {
      api.teams.set(id, settings.team_allies)
      return settings
    })
    const first = render(<App />)
    fireEvent.click(await screen.findByText('Open 1036'))
    await act(async () => {})
    fireEvent.click(screen.getByLabelText('Ally 1'))
    fireEvent.click(screen.getByRole('button', { name: 'C1004' }))
    await act(async () => {})
    expect(api.teams.get('p1036')).toEqual([{ character_id: 1004 }])

    fireEvent.click(screen.getByText('Characters'))
    fireEvent.click(screen.getByText('Open 1036'))
    expect(screen.getByLabelText('Ally 1')).toHaveTextContent('C1004')

    first.unmount()
    render(<App />)
    fireEvent.click(await screen.findByText('Open 1036'))
    await act(async () => {})
    expect(screen.getByLabelText('Ally 1')).toHaveTextContent('C1004')
  })

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
