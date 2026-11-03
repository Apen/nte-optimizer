import { useEffect, useState } from 'react'
import { ArrowLeft } from 'lucide-react'

import { CharacterGameState } from '../../wailsjs/go/main/DesktopApp'
import { ArcEffectDescription, AssetImage, CartridgePieceCard, GameEffectText, ModulePieceCard, type Owner } from '../components/equipment-cards'
import { Button } from '../components/ui/button'
import { PassiveSummary } from '../components/passive-summary'
import { Card, CardContent } from '../components/ui/card'
import { errorMessage } from '../lib/errors'
import { CurrentGameStats } from '../components/current-game-stats'
import { currentIntlLocale, t, type Locale } from '../i18n'
import { presentationName, usePresentation } from '../presentation'
import type { CharacterGameSnapshot, InventoryArc, InventoryCartridge } from '../types'

export function CharacterStatePage({ characterID, locale, onBack }: { characterID: number; locale: Locale; onBack: () => void }) {
  const [snapshot, setSnapshot] = useState<CharacterGameSnapshot>()
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setSnapshot(undefined)
    setError('')
    CharacterGameState(characterID, locale)
      .then(value => { if (active) setSnapshot(value) })
      .catch(value => { if (active) setError(errorMessage(value)) })
    return () => { active = false }
  }, [characterID, locale])

  return <div><Button variant="secondary" className="mb-4" onClick={onBack}><ArrowLeft className="mr-2 size-4" />{t('back_characters')}</Button>{error ? <p role="alert" className="rounded-xl border border-red-900 bg-red-950/40 p-4 text-red-200">{error}</p> : snapshot ? <CurrentGameStateView snapshot={snapshot} /> : <p className="py-16 text-center text-slate-400">{t('loading_imported_state')}</p>}</div>
}

function CurrentGameStateView({ snapshot }: { snapshot: CharacterGameSnapshot }) {
  const presentation = usePresentation()
  const character = snapshot.character
  const skills = (character.skills || []).map(skill => ({ ...skill, category: presentationName(presentation.abilities, skill.abilityId, skill.category) }))
  const observed = character.observedAtLogin
  const activeEquipmentBuffs = observed?.activeEquipmentBuffs || []
  const skillNames: Record<string, string> = { 'Ability.Melee': t('skill_basic_attack'), 'Ability.Skill': t('skill_ability'), 'Ability.UltraSkill': t('skill_ultimate'), 'Ability.QTE': t('skill_qte') }

  return <div className="grid gap-5"><section className="relative overflow-hidden rounded-2xl border border-slate-700 bg-gradient-to-br from-slate-800 via-slate-900 to-slate-800 shadow-lg shadow-black/20"><div className="relative grid min-h-72 md:grid-cols-[18rem_1fr]"><div className="relative overflow-hidden border-b border-slate-700 bg-black/20 md:border-b-0 md:border-r"><img className="absolute inset-0 h-full w-full scale-110 object-cover object-top opacity-25 blur-xl" src={`/game_ui/characters/${character.characterId}.png`} alt="" /><img className="relative mx-auto h-72 w-full object-contain object-bottom drop-shadow-2xl" src={`/game_ui/characters/${character.characterId}.png`} alt={character.name} /></div><div className="grid content-between gap-5 p-6"><div><p className="eyebrow">{t('state_in_game')}</p><div className="mt-2 flex flex-wrap items-end justify-between gap-3"><div><h1 className="text-4xl font-black tracking-tight">{character.name}</h1><p className="mt-1 text-slate-300">{t('level')} {character.level} · {t('breakthrough')} {character.breakthroughLevel}{character.bondLevel !== undefined ? ` · ${t('bond_level')} ${character.bondLevel}` : ''}</p></div>{snapshot.imported_at && <small className="text-slate-500">{t('import_date', { date: new Date(snapshot.imported_at).toLocaleString(currentIntlLocale()) })}</small>}</div></div>{(character.recordedAwakeningLevels?.length ?? 0) > 0 && <div>
  <p className="mb-2 text-xs font-semibold text-amber-300">{t('awakenings')}</p>
  <p className="text-sm font-semibold text-amber-300">{character.recordedAwakeningLevels?.map(level => `${t('awakening_short')}${level}`).join(', ')}</p>
</div>}<div className="grid grid-cols-2 gap-2 lg:grid-cols-4">{skills.map(skill => <div className="rounded-xl border border-white/10 bg-black/20 p-3 backdrop-blur" key={skill.abilityId}><small className="text-slate-400">{skillNames[skill.category] || skill.category}</small><strong className="mt-1 block text-lg">{t('skill_level', { level: skill.level })}</strong></div>)}</div><PassiveSummary references={character.passiveReferences} cards /></div></div></section>
    <CurrentGameStats observedPanelStats={snapshot.observed_panel_stats} calculatedStats={snapshot.stats} />
    <section><h3 className="mb-3 text-lg font-bold">{t('equipped_gear')}</h3><div className="grid gap-4 xl:grid-cols-2">{snapshot.weapon ? <CurrentWeaponCard weapon={snapshot.weapon} /> : <EmptyGear label={t('no_arc_equipped')} />} {snapshot.cartridges.length ? snapshot.cartridges.map(item => <CurrentCartridgeCard key={item.local_id} item={item} owner={{ id: character.characterId, name: character.name }} />) : <EmptyGear label={t('no_cartridge_equipped')} />}</div></section>
    <section><div className="mb-3 flex items-end justify-between"><h3 className="text-lg font-bold">{t('equipped_modules')}</h3><span className="text-xs text-slate-500">{snapshot.modules.length} {snapshot.modules.length === 1 ? t('piece_one') : t('piece_many')}</span></div><div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{snapshot.modules.map((item, index) => <ModulePieceCard key={item.local_id} item={item} title={`${t('module_label', { number: index + 1 })} · ${presentationName(presentation.geometries, item.geometry)}`} meta={`${t('level')} ${item.level}${item.equipped_placement ? ` · ${t('row')} ${item.equipped_placement.row}, ${t('column')} ${item.equipped_placement.column}` : ''}`} ownerCharacter={{ id: character.characterId, name: character.name }} />)}{!snapshot.modules.length && <EmptyGear label={t('no_module_equipped')} />}</div></section>
    {activeEquipmentBuffs.length > 0 && <section className="rounded-xl border border-emerald-900/60 bg-emerald-950/20 p-4"><h3 className="mb-2 font-bold text-emerald-300">{t('observed_bonuses')}</h3>{activeEquipmentBuffs.map(effect => { const description = presentation.set_effects?.[`${effect.setId}_${effect.pieces}`] || effect.effect; return <div className="mb-3 last:mb-0" key={`${effect.setId}-${effect.buff}`}><b className="text-sm">{effect.setName}</b><GameEffectText text={description} className="mt-1 text-sm text-slate-300" /></div> })}</section>}
  </div>
}

function CurrentWeaponCard({ weapon }: { weapon: InventoryArc }) {
  return <Card><CardContent className="flex items-center gap-4 pt-5"><AssetImage source={`/game_ui/forks/${encodeURIComponent(weapon.forkId)}.png`} label={weapon.name} className="size-20 object-contain" /><div><small className="text-slate-500">{t('equipped_arc')}</small><strong className="block text-lg">{weapon.name || weapon.forkId}</strong><span className="text-sm text-slate-400">{t('level')} {weapon.level} · {t('breakthrough').toLocaleLowerCase()} {weapon.breakthrough} · {t('refinement')} {weapon.star}</span><ArcEffectDescription forkID={weapon.forkId} star={weapon.star} /></div></CardContent></Card>
}

function CurrentCartridgeCard({ item, owner }: { item: InventoryCartridge; owner: Owner }) {
  return <CartridgePieceCard item={item} title={item.set_name || item.set_id} meta={`${t('level')} ${item.level}`} ownerCharacter={owner} />
}

function EmptyGear({ label }: { label: string }) {
  return <div className="grid min-h-28 place-items-center rounded-xl border border-dashed border-slate-700 text-sm text-slate-500">{label}</div>
}
