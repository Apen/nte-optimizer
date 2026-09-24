import { useEffect, useMemo, useRef, useState } from 'react'

import { ModulePieceCard } from './equipment-cards'
import { t } from '../i18n'
import { usePresentation } from '../presentation'
import type { Result } from '../types'

const moduleColors = ['bg-slate-700', 'bg-red-500', 'bg-cyan-500', 'bg-emerald-500', 'bg-violet-500', 'bg-amber-500', 'bg-blue-500', 'bg-fuchsia-500']

export function ConsoleGrid({ result }: { result: Result }) {
  const [hoveredModuleID, setHoveredModuleID] = useState<string>()
  const [pinnedModuleID, setPinnedModuleID] = useState<string>()
  const previewRef = useRef<HTMLDivElement>(null)
  const { geometries } = usePresentation()
  const moduleByID = useMemo(() => new Map(result.modules.map(entry => [entry.module.local_id, entry])), [result.modules])
  const byCell = useMemo(() => {
    const map = new Map<string, { moduleID: string; number: number }>()
    result.solution.placements.forEach((placement, index) => placement.cells.forEach(cell => map.set(`${cell.x},${cell.y}`, { moduleID: placement.module_id, number: index + 1 })))
    return map
  }, [result.solution.placements])
  const playable = new Set(result.grid.playable.map(cell => `${cell.x},${cell.y}`))
  const activeModuleID = hoveredModuleID ?? pinnedModuleID
  const activeModule = activeModuleID ? moduleByID.get(activeModuleID) : undefined
  const activeNumber = activeModuleID ? result.solution.placements.findIndex(placement => placement.module_id === activeModuleID) + 1 : 0
  const cells = []

  useEffect(() => {
    if (!pinnedModuleID) return
    const dismissOutside = (event: PointerEvent) => {
      if (!(event.target instanceof Element)) return
      if (event.target.closest('[aria-controls="console-grid-module-preview"]') || previewRef.current?.contains(event.target)) return
      setPinnedModuleID(undefined)
      setHoveredModuleID(undefined)
    }
    document.addEventListener('pointerdown', dismissOutside)
    return () => document.removeEventListener('pointerdown', dismissOutside)
  }, [pinnedModuleID])

  for (let y = 0; y < result.grid.height; y++) {
    for (let x = 0; x < result.grid.width; x++) {
      const key = `${x},${y}`
      const placement = byCell.get(key)
      const module = placement ? moduleByID.get(placement.moduleID) : undefined
      const content = placement?.number || ''
      const className = `grid size-12 place-items-center rounded-lg font-black transition ${playable.has(key) ? moduleColors[placement?.number || 0] : 'border border-dashed border-slate-700 bg-slate-950'} ${placement && activeModuleID && placement.moduleID === activeModuleID ? 'ring-2 ring-white/80 ring-offset-1 ring-offset-slate-950' : ''}`

      cells.push(module && placement
        ? <button key={key} type="button" className={`${className} cursor-help focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300`} aria-label={t('module_label', { number: placement.number })} aria-controls="console-grid-module-preview" onPointerEnter={() => setHoveredModuleID(placement.moduleID)} onFocus={() => setHoveredModuleID(placement.moduleID)} onClick={() => setPinnedModuleID(placement.moduleID)}>{content}</button>
        : <div key={key} className={className}>{content}</div>)
    }
  }

  return <section className="relative" onPointerLeave={() => setHoveredModuleID(undefined)} onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setHoveredModuleID(undefined) }}>
    <h2 className="mb-3 font-bold">{t('console_grid')}</h2>
    <div className="grid gap-1.5" style={{ gridTemplateColumns: `repeat(${result.grid.width},3rem)` }}>{cells}</div>
    <div className="relative mt-3 min-h-4">
      <p className="text-xs text-slate-500">{t('hover_grid_piece_hint')}</p>
      <div ref={previewRef} id="console-grid-module-preview" className="absolute left-0 top-full z-30 mt-2 w-[min(24rem,calc(100vw-2rem))] shadow-2xl" aria-live="polite">
        {activeModule && <ModulePieceCard item={activeModule.module} title={t('module_label', { number: activeNumber })} meta={geometries[activeModule.module.geometry] || activeModule.module.geometry} className="border-cyan-700 bg-slate-950" />}
      </div>
    </div>
  </section>
}
