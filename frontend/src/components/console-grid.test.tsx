import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ConsoleGrid } from './console-grid'
import type { Result } from '../types'

const result = {
  grid: { width: 3, height: 1, playable: [{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 2, y: 0 }] },
  modules: [
    { module: { local_id: 'module-other', geometry: 'triangle', main_stats: [{ property_id: 'HPMaxUp', value: 0.1, percent: true }], sub_stats: [] }, breakdown: { total: 0 } },
    { module: { local_id: 'module-a', geometry: 'square', main_stats: [{ property_id: 'AtkUp', value: 0.12, percent: true }], sub_stats: [{ property_id: 'CritBase', value: 0.032, percent: true }] }, breakdown: { total: 0 } },
  ],
  solution: { placements: [{ module_id: 'module-a', cells: [{ x: 0, y: 0 }, { x: 1, y: 0 }] }] },
} as Result

describe('ConsoleGrid', () => {
  it('shows the placed module stats when its grid piece is hovered', () => {
    const { container } = render(<ConsoleGrid result={result} />)

    expect(container.querySelector('.grid > div')?.className).not.toContain('ring-2')

    fireEvent.pointerEnter(screen.getAllByRole('button', { name: 'module_label' })[0])

    expect(screen.getByText('AtkUp')).toBeTruthy()
    expect(screen.getByText('CritBase')).toBeTruthy()
    expect(screen.queryByText('HPMaxUp')).toBeNull()
    expect(screen.getByText('AtkUp').closest('#console-grid-module-preview')?.className).toContain('absolute')
    expect(screen.getByText('AtkUp').closest('.nte-card')?.className).toContain('bg-slate-950')
    expect(container.querySelector('.grid > div')?.className).not.toContain('ring-2')
  })

  it('keeps the module preview open after clicking a piece until an outside click', () => {
    render(<ConsoleGrid result={result} />)
    const cell = screen.getAllByRole('button', { name: 'module_label' })[0]

    fireEvent.click(cell)
    fireEvent.pointerLeave(cell.closest('section')!)
    expect(screen.getByText('AtkUp')).toBeTruthy()

    fireEvent.pointerDown(document.body)
    expect(screen.queryByText('AtkUp')).toBeNull()
  })

  it('dismisses a pinned preview when clicking an empty grid cell', () => {
    const { container } = render(<ConsoleGrid result={result} />)

    fireEvent.click(screen.getAllByRole('button', { name: 'module_label' })[0])
    expect(screen.getByText('AtkUp')).toBeTruthy()

    fireEvent.pointerDown(container.querySelector('.grid > div')!)
    expect(screen.queryByText('AtkUp')).toBeNull()
  })
})
