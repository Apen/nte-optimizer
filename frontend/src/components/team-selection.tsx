import { Plus } from 'lucide-react'
import { useState, type ReactNode } from 'react'

import { t } from '../i18n'
import type { InventoryArc, TeamAlly, WorkspaceCharacter } from '../types'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

export function teamArcKey(arc: InventoryArc) {
  return `${arc.id.solt}:${arc.id.serial}`
}

type Props = {
  children?: ReactNode
  activeID: number
  characters: WorkspaceCharacter[]
  arcs: InventoryArc[]
  allies: TeamAlly[]
  onChange: (allies: TeamAlly[]) => void
}

export function TeamSelection({ activeID, characters, arcs, allies, onChange, children }: Props) {
  const active = characters.find(character => character.character_id === activeID)
  if (!active) return null

  function changeAlly(index: number, characterID: number) {
    if (!characterID) {
      onChange(allies.filter((_, position) => position !== index))
    } else if (index === allies.length) {
      onChange([...allies, { character_id: characterID }])
    } else {
      onChange(allies.map((ally, position) => position === index ? { character_id: characterID } : ally))
    }
  }

  return <details className="mt-3 rounded-xl border border-slate-700/80 bg-slate-950/30 p-3">
    <summary className="cursor-pointer text-sm font-bold">{t('team_setup')} · {allies.length}/3</summary>
    <div className="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
      <div className="flex min-w-0 items-center gap-3 rounded-xl border border-pink-500/40 bg-pink-950/25 p-3">
        <Portrait character={active} />
        <div className="min-w-0">
          <p className="text-xs font-semibold text-pink-300">{t('team_active_badge')}</p>
          <strong className="block truncate text-sm">{active.name}</strong>
        </div>
      </div>
      {Array.from({ length: 3 }, (_, index) => {
        const ally = allies[index]
        const character = characters.find(item => item.character_id === ally?.character_id)
        const gear = character?.team_gear
        const arc = gear?.arc_id ? arcs.find(item => teamArcKey(item) === gear.arc_id) : undefined
        const available = characters.filter(item => item.character_id !== activeID &&
          !allies.some((member, position) => position !== index && member.character_id === item.character_id))
        return <div key={index} className="grid min-w-0 content-start gap-2 rounded-xl border border-slate-700 bg-slate-900/60 p-3">
          <div className="flex min-w-0 items-center gap-2">
            {character ? <Portrait character={character} /> : <div className="grid size-10 shrink-0 place-items-center rounded-lg border border-dashed border-slate-600 text-slate-400"><Plus aria-hidden="true" className="size-4" /></div>}
            <AllyPicker label={t('team_ally', { number: index + 1 })} selected={character} characters={available}
              disabled={index > allies.length || !available.length} onSelect={id => changeAlly(index, id)} />
          </div>
          {character && <div className="grid gap-1 text-xs text-slate-300">
            <p className="text-slate-400">{t(gear?.saved ? 'team_saved_gear' : 'team_scanned_gear')}</p>
            <p>{arc ? `${arc.name || arc.forkId} · ★${arc.star}` : t('no_arc')}</p>
            <p>{gear?.set_name ? `${gear.set_name} · ${t('team_set_pieces', { count: gear.set_pieces ?? 0 })}` : t('team_no_set')}</p>
          </div>}
        </div>
      })}
    </div>
    {children}
  </details>
}

function AllyPicker({ label, selected, characters, disabled, onSelect }: {
  label: string
  selected?: WorkspaceCharacter
  characters: WorkspaceCharacter[]
  disabled: boolean
  onSelect: (id: number) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const search = query.trim().toLocaleLowerCase()
  const matches = characters.filter(character => character.name.toLocaleLowerCase().includes(search))

  function choose(id: number) {
    onSelect(id)
    setOpen(false)
  }

  return <Dialog open={open} onOpenChange={value => { setOpen(value); if (value) setQuery('') }}>
    <div className="grid min-w-0 flex-1 gap-1 text-xs text-slate-400">
      <span>{label}</span>
      <DialogTrigger asChild>
        <button type="button" aria-label={label} disabled={disabled}
          className="min-h-10 truncate rounded-lg border border-slate-700 bg-slate-800 px-3 text-left text-sm text-slate-100 hover:border-slate-500 disabled:opacity-50">
          {selected?.name ?? t('team_no_ally')}
        </button>
      </DialogTrigger>
    </div>
    <DialogContent title={label} className="max-w-2xl">
      <h2 className="pr-10 text-lg font-bold">{label}</h2>
      <input autoFocus type="search" value={query} onChange={event => setQuery(event.target.value)}
        aria-label={t('team_search_allies')} placeholder={t('team_search_allies')} className="mt-3 w-full" />
      <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
        {matches.map(character => <button type="button" key={character.character_id} onClick={() => choose(character.character_id)}
          aria-pressed={selected?.character_id === character.character_id}
          className="flex min-w-0 items-center gap-2 rounded-lg border border-slate-700 bg-slate-900 p-2 text-left text-sm hover:border-pink-400 aria-pressed:border-pink-400">
          <Portrait character={character} />
          <span className="break-words">{character.name}</span>
        </button>)}
      </div>
      {selected && <button type="button" className="mt-3 rounded-lg border border-slate-700 px-3 py-2 text-sm" onClick={() => choose(0)}>{t('team_no_ally')}</button>}
    </DialogContent>
  </Dialog>
}

function Portrait({ character }: { character: WorkspaceCharacter }) {
  return <img className="size-10 shrink-0 rounded-lg bg-slate-800 object-cover" src={`/game_ui/characters/${character.character_id}.png`} alt="" />
}
