import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { CharactersPage } from './characters-page'

const characters = [
  { character_id: 1, name: 'Astra', level: 80, awaken_level: 0, priority: 1 },
  { character_id: 2, name: 'Beryl', level: 80, awaken_level: 1, priority: 2 },
  { character_id: 3, name: 'Cinder', level: 80, awaken_level: 2, priority: 3 },
]

describe('CharactersPage', () => {
  it('reorders characters by dragging one onto another row', () => {
    const onReorder = vi.fn()
    render(<CharactersPage characters={characters} selected={0} onSelect={vi.fn()} onMove={vi.fn()} onReorder={onReorder} onBuild={vi.fn()} onState={vi.fn()} />)

    const sourceRow = screen.getByText('Astra').closest('.character-row')!
    const targetRow = screen.getByText('Cinder').closest('.character-row')!

    fireEvent.dragStart(sourceRow, { dataTransfer: { effectAllowed: 'none', setData: vi.fn() } })
    fireEvent.dragOver(targetRow, { dataTransfer: { dropEffect: 'none' } })
    fireEvent.drop(targetRow, { dataTransfer: { getData: () => '' } })

    expect(onReorder).toHaveBeenCalledWith([2, 3, 1])
  })

  it('does not start dragging from an action button', () => {
    const dataTransfer = { effectAllowed: 'none', setData: vi.fn() }
    render(<CharactersPage characters={characters} selected={0} onSelect={vi.fn()} onMove={vi.fn()} onReorder={vi.fn()} onBuild={vi.fn()} onState={vi.fn()} />)

    fireEvent.dragStart(screen.getAllByRole('button', { name: 'create_build' })[0], { dataTransfer })

    expect(dataTransfer.setData).not.toHaveBeenCalled()
  })
})
