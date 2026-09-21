import { useEffect, useState } from 'react'
import { ArrowLeft } from 'lucide-react'

import { CharacterGameState } from '../../wailsjs/go/main/DesktopApp'
import { AssetImage, CartridgePieceCard, ModulePieceCard, type Owner } from '../components/equipment-cards'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import { currentIntlLocale, t, type Locale } from '../i18n'
import { formatStat } from '../lib/format'
import { presentationName, usePresentation } from '../presentation'
import type { CharacterGameSnapshot, InventoryArc, InventoryCartridge, StatSummary } from '../types'

export function CharacterStatePage({ characterID, locale, onBack }: { characterID: number; locale: Locale; onBack: () => void }) {
  const [snapshot, setSnapshot] = useState<CharacterGameSnapshot>()
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setSnapshot(undefined)
    setError('')
    CharacterGameState(characterID, locale)
      .then(value => { if (active) setSnapshot(value as CharacterGameSnapshot) })
      .catch(value => { if (active) setError(String(value)) })
    return () => { active = false }
  }, [characterID, locale])

  return <div><Button variant="secondary" className="mb-4" onClick={onBack}><ArrowLeft className="mr-2 size-4" />{t('back_characters')}</Button>{error ? <p className="rounded-xl border border-red-900 bg-red-950/40 p-4 text-red-200">{error}</p> : snapshot ? <CurrentGameStateView snapshot={snapshot} /> : <p className="py-16 text-center text-slate-400">{t('loading_imported_state')}</p>}</div>
}

function CurrentGameStateView({ snapshot }: { snapshot: CharacterGameSnapshot }) {
  const presentation = usePresentation()
  const character = snapshot.character
  const skills = (character.skills || []).map(skill => ({ ...skill, category: presentationName(presentation.abilities, skill.abilityId, skill.category) }))
  const observed = character.observedAtLogin
  const observedAwakenings = new Set(observed?.activeAwakeningLevels || [])
  const activeEquipmentBuffs = observed?.activeEquipmentBuffs || []
  const awakeningIsActive = (level: number) => observed?.loaded ? observedAwakenings.has(level) : level <= character.awakenLevel
  const skillNames: Record<string, string> = { 'Ability.Melee': t('skill_basic_attack'), 'Ability.Skill': t('skill_ability'), 'Ability.UltraSkill': t('skill_ultimate'), 'Ability.QTE': t('skill_qte') }

  return <div className="grid gap-5"><section className="relative overflow-hidden rounded-2xl border border-indigo-800/70 bg-gradient-to-br from-indigo-950 via-slate-900 to-pink-950 shadow-2xl shadow-black/30"><div className="absolute inset-0 opacity-20" style={{ backgroundImage: 'radial-gradient(circle at 75% 20%, #e879f9 0, transparent 28%), radial-gradient(circle at 20% 80%, #38bdf8 0, transparent 30%)' }} /><div className="relative grid min-h-72 md:grid-cols-[18rem_1fr]"><div className="relative overflow-hidden border-b border-indigo-800/60 bg-black/20 md:border-b-0 md:border-r"><img className="absolute inset-0 h-full w-full scale-110 object-cover object-top opacity-25 blur-xl" src={`/game_ui/characters/${character.characterId}.png`} alt="" /><img className="relative mx-auto h-72 w-full object-contain object-bottom drop-shadow-2xl" src={`/game_ui/characters/${character.characterId}.png`} alt={character.name} /></div><div className="grid content-between gap-5 p-6"><div><p className="eyebrow">{t('state_in_game')}</p><div className="mt-2 flex flex-wrap items-end justify-between gap-3"><div><h1 className="text-4xl font-black tracking-tight">{character.name}</h1><p className="mt-1 text-slate-300">{t('level')} {character.level} · {t('breakthrough')} {character.breakthroughLevel}</p></div>{snapshot.imported_at && <small className="text-slate-500">{t('import_date', { date: new Date(snapshot.imported_at).toLocaleString(currentIntlLocale()) })}</small>}</div></div><div><p className="mb-2 text-[10px] font-black uppercase tracking-wider text-amber-300">{t('awakenings')}</p><div className="flex flex-wrap gap-2">{[1, 2, 3, 4, 5, 6].map(level => <span key={level} className={`grid size-9 place-items-center rounded-full border font-black ${awakeningIsActive(level) ? 'border-amber-400 bg-amber-500 text-slate-950' : 'border-slate-700 bg-slate-950/70 text-slate-600'}`}>{t('awakening_short')}{level}</span>)}</div>{observed?.loaded && <p className="mt-2 text-xs text-slate-400">{t('observed_login_effects')}</p>}</div><div className="grid grid-cols-2 gap-2 lg:grid-cols-4">{skills.map(skill => <div className="rounded-xl border border-white/10 bg-black/20 p-3 backdrop-blur" key={skill.abilityId}><small className="text-slate-400">{skillNames[skill.category] || skill.category}</small><strong className="mt-1 block text-lg">{t('skill_level', { level: skill.level })}</strong></div>)}</div></div></div></section>
    <CurrentGameStats stats={snapshot.stats} />
    <section><h3 className="mb-3 text-lg font-bold">{t('equipped_gear')}</h3><div className="grid gap-4 xl:grid-cols-2">{snapshot.weapon ? <CurrentWeaponCard weapon={snapshot.weapon} /> : <EmptyGear label={t('no_arc_equipped')} />} {snapshot.cartridges.length ? snapshot.cartridges.map(item => <CurrentCartridgeCard key={item.local_id} item={item} owner={{ id: character.characterId, name: character.name }} />) : <EmptyGear label={t('no_cartridge_equipped')} />}</div></section>
    <section><div className="mb-3 flex items-end justify-between"><h3 className="text-lg font-bold">{t('equipped_modules')}</h3><span className="text-xs text-slate-500">{snapshot.modules.length} {snapshot.modules.length === 1 ? t('piece_one') : t('piece_many')}</span></div><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{snapshot.modules.map((item, index) => <ModulePieceCard key={item.local_id} item={item} title={`${t('module_label', { number: index + 1 })} · ${presentationName(presentation.geometries, item.geometry)}`} meta={`${t('level')} ${item.level}${item.equipped_placement ? ` · ${t('row')} ${item.equipped_placement.row}, ${t('column')} ${item.equipped_placement.column}` : ''}`} ownerCharacter={{ id: character.characterId, name: character.name }} />)}{!snapshot.modules.length && <EmptyGear label={t('no_module_equipped')} />}</div></section>
    {activeEquipmentBuffs.length > 0 && <section className="rounded-xl border border-emerald-900/60 bg-emerald-950/20 p-4"><h3 className="mb-2 font-bold text-emerald-300">{t('observed_bonuses')}</h3>{activeEquipmentBuffs.map(effect => <p className="text-sm text-slate-300" key={`${effect.setId}-${effect.buff}`}><b>{effect.setName}</b> · {effect.pieces} {t('piece_many')} — {effect.effect}</p>)}</section>}
  </div>
}

function CurrentGameStats({ stats }: { stats?: StatSummary }) {
  const { stats: labels } = usePresentation()
  if (!stats) return <Card><CardContent className="py-8 text-center text-sm text-slate-500">{t('no_stats_available')}</CardContent></Card>
  const entries = Object.entries(stats.derived || {}).filter(([, value]) => Number.isFinite(value)).sort(([left], [right]) => (labels[left] || left).localeCompare(labels[right] || right, currentIntlLocale()))
  return <Card><CardHeader><div><h2 className="text-xl font-bold">{t('current_stats')}</h2><p className="mt-1 text-sm text-slate-400">{t('current_stats_description')}</p></div></CardHeader><CardContent>{stats.completeness === 'partial' && <p className="mb-4 rounded-lg border border-amber-900/60 bg-amber-950/30 p-3 text-xs text-amber-200">{t('estimated_stats')}</p>}<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">{entries.map(([key, value]) => { const conditional = stats.derived_conditional?.[key]; const hasConditional = conditional !== undefined && Math.abs(conditional - value) > .00001; return <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2" key={key}><span className="block text-xs text-slate-500">{labels[key] || key}</span><strong className="block text-lg tabular-nums text-slate-100">{formatStat(key, value)}</strong>{hasConditional && <small className="block text-[10px] text-amber-300">{t('max_effects_value', { value: formatStat(key, conditional) })}</small>}</div> })}</div></CardContent></Card>
}

function CurrentWeaponCard({ weapon }: { weapon: InventoryArc }) {
  return <Card><CardContent className="flex items-center gap-4 pt-5"><AssetImage source={`/game_ui/forks/${encodeURIComponent(weapon.forkId)}.png`} label={weapon.name} className="size-20 object-contain" /><div><small className="text-slate-500">{t('equipped_arc')}</small><strong className="block text-lg">{weapon.name || weapon.forkId}</strong><span className="text-sm text-slate-400">{t('level')} {weapon.level} · {t('breakthrough').toLocaleLowerCase()} {weapon.breakthrough} · {t('refinement')} {weapon.star}</span></div></CardContent></Card>
}

function CurrentCartridgeCard({ item, owner }: { item: InventoryCartridge; owner: Owner }) {
  return <CartridgePieceCard item={item} title={item.set_name || item.set_id} meta={`${t('level')} ${item.level}`} ownerCharacter={owner} />
}

function EmptyGear({ label }: { label: string }) {
  return <div className="grid min-h-28 place-items-center rounded-xl border border-dashed border-slate-700 text-sm text-slate-500">{label}</div>
}
