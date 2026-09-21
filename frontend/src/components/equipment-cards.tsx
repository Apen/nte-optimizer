import { useEffect, useState, type ReactNode } from 'react'

import { Card, CardContent } from './ui/card'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'
import { t } from '../i18n'
import { formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { InventoryCartridge, InventoryModule, Stat } from '../types'

export type Owner = { id: number; name: string }

type PieceCardProps = {
  title: string
  meta?: string
  owner?: string
  ownerCharacter?: Owner
  leading?: ReactNode
  trailing?: ReactNode
  actions?: ReactNode
  className?: string
}

export function OwnerBadge({ owner }: { owner: Owner }) {
  return <span className="mt-1 inline-flex h-6 max-w-full items-center gap-1.5 rounded-full border border-amber-800/70 bg-amber-950/50 pr-2 text-xs font-bold text-amber-200"><img className="size-6 rounded-full object-cover object-top" src={`/game_ui/characters/${owner.id}.png`} alt="" /><span className="truncate">{owner.name}</span></span>
}

function PieceCard({ tone, image, title, meta, owner, ownerCharacter, mainStats, subStats, leading, trailing, actions, className = '' }: { tone: 'module' | 'cartridge'; image: ReactNode; mainStats: Stat[]; subStats: Stat[] } & PieceCardProps) {
  const cartridge = tone === 'cartridge'
  return <Card className={`overflow-hidden ${className}`}><div className={`flex items-center gap-3 border-b bg-gradient-to-r to-slate-950 p-3 ${cartridge ? 'border-violet-900/60 from-violet-950/80' : 'border-orange-900/60 from-orange-950/80'}`}>{leading}{image}<div className="min-w-0 flex-1"><strong className={`block ${cartridge ? 'text-violet-100' : 'text-orange-100'}`}>{title}</strong>{meta && <small className="block text-slate-400">{meta}</small>}{ownerCharacter ? <OwnerBadge owner={ownerCharacter} /> : owner && <small className="block h-6 leading-6 text-emerald-400">{owner}</small>}</div>{trailing}</div><CardContent className="pt-3"><Stats title={cartridge ? t('main_attribute') : t('main_attributes')} stats={mainStats} /><Stats title={t('secondary_attributes')} stats={subStats} />{actions && <div className="mt-3 flex gap-2 border-t border-slate-800 pt-3">{actions}</div>}</CardContent></Card>
}

type ModulePiece = Pick<InventoryModule, 'game_item_id' | 'geometry' | 'main_stats' | 'sub_stats'>
type CartridgePiece = Pick<InventoryCartridge, 'game_item_id' | 'main_stats' | 'sub_stats'>

export function ModulePieceCard({ item, ...props }: { item: ModulePiece } & PieceCardProps) {
  const { geometries } = usePresentation()
  const geometry = geometries[item.geometry] || item.geometry
  return <PieceCard tone="module" image={<ModuleIcon itemID={item.game_item_id} label={geometry} />} mainStats={item.main_stats} subStats={item.sub_stats} {...props} />
}

export function CartridgePieceCard({ item, ...props }: { item: CartridgePiece } & PieceCardProps) {
  return <PieceCard tone="cartridge" image={<AssetImage source={`/game_ui/equipment/core/${encodeURIComponent(item.game_item_id || '')}.png`} label={props.title} className="size-14 object-contain" />} mainStats={item.main_stats} subStats={item.sub_stats} {...props} />
}

function Stats({ title, stats }: { title: string; stats: Stat[] }) {
  const { stats: labels } = usePresentation()
  if (!stats.length) return null
  return <div className="mt-2 border-t border-slate-800 pt-2"><p className="mb-1 text-[10px] font-black uppercase tracking-wider text-slate-500">{title}</p><div className="grid gap-1 sm:grid-cols-2">{stats.map((stat, index) => <div className="flex justify-between rounded bg-slate-950 px-2 py-1.5 text-xs" key={`${stat.property_id}-${index}`}><span className="text-slate-400">{labels[stat.property_id] || stat.property_id}</span><b>{stat.percent ? `${(stat.value * 100).toFixed(1)} %` : formatStat(stat.property_id, stat.value)}</b></div>)}</div></div>
}

function ModuleIcon({ itemID, label }: { itemID?: string; label: string }) {
  if (!itemID) return <div className="grid size-14 shrink-0 place-items-center rounded-lg border border-dashed border-slate-700 text-[9px] text-slate-600">{t('no_icon')}</div>
  return <AssetImage source={`/game_ui/equipment/module/${encodeURIComponent(itemID)}.png`} label={t('module_shape_alt', { shape: label })} className="size-12 object-contain" />
}

export function AssetImage({ source, label, className }: { source: string; label: string; className: string }) {
  const [missing, setMissing] = useState(false)
  useEffect(() => setMissing(false), [source])
  if (missing) return null
  return <Dialog><DialogTrigger asChild><button className="shrink-0 rounded-lg bg-black/30 p-1" title={t('enlarge_image')}><img className={`${className} transition hover:scale-105`} src={source} alt={label} onError={() => setMissing(true)} /></button></DialogTrigger><DialogContent><img className="mx-auto max-h-[75vh] w-full object-contain" src={source} alt={label} /><p className="mt-3 text-center text-sm text-slate-400">{label}</p></DialogContent></Dialog>
}
