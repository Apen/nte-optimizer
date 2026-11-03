import { fireEvent, render, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { expect, it } from 'vitest'

import { applyLocalization } from '../i18n'
import type { InventoryArc, TeamAlly, WorkspaceCharacter } from '../types'
import { TeamSelection } from './team-selection'

it('selects distinct allies and shows their equipment without an Arc selector', () => {
  applyLocalization('en', {
    locale: 'en',
    ui: { team_ally: 'Ally {number}', team_no_ally: 'No ally', team_search_allies: 'Search allies', team_active_badge: 'Active member', team_saved_gear: 'Saved build', team_set_pieces: '{count} matched pieces' },
    stats: {}, qualities: {}, geometries: {}, damage: {}, stat_sources: {},
  })
  const characters: WorkspaceCharacter[] = ['Active', 'First', 'Second', 'Third', 'Fourth'].map((name, index) => ({
    character_id: index + 1, name, level: 80, awaken_level: 0, priority: index,
  }))
  const arc: InventoryArc = {
    id: { solt: 1, serial: 2 }, forkId: 'synthetic-arc', name: 'Synthetic Arc',
    quality: 'orange', level: 80, breakthrough: 6, star: 1, compatible_characters: characters,
  }
  characters[1].team_gear = { arc_id: '1:2', saved: true, set_name: 'Synthetic Set', set_pieces: 4 }
  function Builder() {
    const [allies, setAllies] = useState<TeamAlly[]>([])
    return <TeamSelection activeID={1} characters={characters} arcs={[arc]} allies={allies} onChange={setAllies} />
  }

  render(<Builder />)
  expect(screen.getByText('Active member')).toBeInTheDocument()
  const first = screen.getByLabelText('Ally 1')
  fireEvent.click(first)
  const picker = within(screen.getByRole('dialog', { name: 'Ally 1' }))
  expect(picker.queryByRole('button', { name: 'Active' })).toBeNull()
  fireEvent.change(picker.getByRole('searchbox'), { target: { value: 'fir' } })
  expect(picker.queryByRole('button', { name: 'Second' })).toBeNull()
  const firstChoice = picker.getByRole('button', { name: 'First' })
  expect(firstChoice.querySelector('img')).toHaveAttribute('src', '/game_ui/characters/2.png')
  fireEvent.click(firstChoice)
  expect(screen.getByText('Synthetic Arc · ★1')).toBeInTheDocument()
  expect(screen.getByText('Synthetic Set · 4 matched pieces')).toBeInTheDocument()
  expect(screen.getByText('Saved build')).toBeInTheDocument()
  expect(document.querySelector('img[src="/game_ui/characters/2.png"]')).toBeInTheDocument()

  fireEvent.click(screen.getByLabelText('Ally 2'))
  const secondPicker = within(screen.getByRole('dialog', { name: 'Ally 2' }))
  expect(secondPicker.queryByRole('button', { name: 'First' })).toBeNull()
  fireEvent.click(secondPicker.getByRole('button', { name: 'Second' }))
  fireEvent.click(screen.getByLabelText('Ally 3'))
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Ally 3' })).getByRole('button', { name: 'Third' }))
  expect(screen.queryByLabelText('Ally 4')).toBeNull()
  expect(screen.queryByLabelText('Arc for ally 1')).toBeNull()
  fireEvent.click(first)
  fireEvent.click(within(screen.getByRole('dialog', { name: 'Ally 1' })).getByRole('button', { name: 'No ally' }))
  expect(screen.queryByText('Synthetic Arc · ★1')).toBeNull()
})
