import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, Eye, Search } from 'lucide-react'

import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import { t } from '../i18n'
import type { WorkspaceCharacter } from '../types'

type CharactersPageProps = {
  characters: WorkspaceCharacter[]
  selected: number
  onSelect: (id: number) => void
  onMove: (id: number, direction: -1 | 1) => void
  onReorder: (ids: number[]) => void
  onBuild: (id: number) => void
  onState: (id: number) => void
}

export function CharactersPage({ characters, selected, onSelect, onMove, onReorder, onBuild, onState }: CharactersPageProps) {
  const [query, setQuery] = useState('')
  const [draggedCharacter, setDraggedCharacter] = useState<number>()
  const [dragOverCharacter, setDragOverCharacter] = useState<number>()
  const needle = query.trim().toLocaleLowerCase()
  const visible = useMemo(() => characters.filter(character => character.name.toLocaleLowerCase().includes(needle)), [characters, needle])
  const positions = useMemo(() => new Map(characters.map((character, index) => [character.character_id, index])), [characters])

  function reorderCharacters(sourceID: number, targetID: number) {
    const ids = characters.map(character => character.character_id)
    const sourceIndex = ids.indexOf(sourceID)
    const targetIndex = ids.indexOf(targetID)
    if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return
    const [movedID] = ids.splice(sourceIndex, 1)
    ids.splice(targetIndex, 0, movedID)
    onReorder(ids)
  }

  return <>
    <header className="mb-7"><p className="eyebrow">{t('character_section')}</p><h1 className="text-4xl font-black tracking-tight sm:text-5xl">{t('team_priority_title')}</h1><p className="mt-2 max-w-2xl text-slate-400">{t('team_priority_description')}</p></header>
    <Card><CardHeader><div className="flex flex-wrap items-center justify-between gap-4"><div><h2 className="text-xl font-bold">{t('character_order_title')}</h2><p className="text-sm text-slate-500">{t('character_order_description')}</p></div><label className="relative"><Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-500" /><input className="h-10 w-64 rounded-lg border border-slate-700 bg-slate-950 pl-9 pr-3 text-sm outline-none focus:border-pink-500" placeholder={t('search_character_placeholder')} value={query} onChange={event => setQuery(event.target.value)} /></label></div></CardHeader><CardContent><div className="grid gap-2">{visible.map(character => {
      const index = positions.get(character.character_id) ?? -1
      const isDragging = draggedCharacter === character.character_id
      const isDragTarget = dragOverCharacter === character.character_id
      return <div key={character.character_id} draggable onClick={() => onSelect(character.character_id)} onDragStart={event => { if ((event.target as HTMLElement).closest('button')) { event.preventDefault(); return }; event.stopPropagation(); setDraggedCharacter(character.character_id); if (event.dataTransfer) { event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', String(character.character_id)) } }} onDragEnd={() => { setDraggedCharacter(undefined); setDragOverCharacter(undefined) }} onDragOver={event => { if (draggedCharacter === undefined || draggedCharacter === character.character_id) return; event.preventDefault(); if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'; setDragOverCharacter(character.character_id) }} onDrop={event => { event.preventDefault(); event.stopPropagation(); const payload = event.dataTransfer?.getData('text/plain'); const sourceID = draggedCharacter ?? (payload ? Number(payload) : undefined); if (sourceID !== undefined && Number.isFinite(sourceID)) reorderCharacters(sourceID, character.character_id); setDraggedCharacter(undefined); setDragOverCharacter(undefined) }} className={`character-row grid cursor-grab active:cursor-grabbing grid-cols-[auto_auto_1fr_auto_auto_auto] items-center gap-3 rounded-xl border p-3 transition ${selected === character.character_id ? 'border-pink-500 bg-pink-950/20' : 'border-slate-800 bg-slate-950/50 hover:border-slate-700'} ${isDragTarget ? 'ring-2 ring-cyan-400/70' : ''} ${isDragging ? 'opacity-50' : ''}`}>
        <span className="grid size-9 place-items-center rounded-full bg-slate-900 font-black text-slate-300">{character.priority}</span>
        <img className="size-14 rounded-xl bg-slate-900 object-cover object-top" src={`/game_ui/characters/${character.character_id}.png`} alt="" />
        <span className="min-w-0"><strong className="block text-base">{character.name}</strong><small className={character.build ? 'text-emerald-400' : 'text-slate-500'}>{character.build ? t('build_saved') : t('no_build_saved')} · {t('level').toLocaleLowerCase()} {character.level} · {t('awakening_short')}{character.awaken_level}</small></span>
        <span className="character-order flex gap-1">
          <button disabled={index === 0} onClick={event => { event.stopPropagation(); onMove(character.character_id, -1) }} className="rounded-lg border border-slate-700 bg-slate-900 p-2 hover:border-pink-500 disabled:opacity-20" title={t('move_up')}><ArrowUp className="size-4" /></button>
          <button disabled={index === characters.length - 1} onClick={event => { event.stopPropagation(); onMove(character.character_id, 1) }} className="rounded-lg border border-slate-700 bg-slate-900 p-2 hover:border-pink-500 disabled:opacity-20" title={t('move_down')}><ArrowDown className="size-4" /></button>
        </span>
        <Button variant="secondary" onClick={event => { event.stopPropagation(); onState(character.character_id) }}><Eye className="mr-2 size-4" />{t('game_state')}</Button>
        <Button onClick={event => { event.stopPropagation(); onBuild(character.character_id) }}>{character.build ? t('view_build') : t('create_build')}</Button>
      </div>
    })}{!visible.length && <p className="py-12 text-center text-slate-500">{t('no_character_match', { query })}</p>}</div></CardContent></Card>
  </>
}
