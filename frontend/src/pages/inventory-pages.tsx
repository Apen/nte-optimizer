import { useMemo, useState, type ReactNode } from 'react'
import { Search, X } from 'lucide-react'

import { AssetImage, CartridgePieceCard, ModulePieceCard, OwnerBadge, type Owner } from '../components/equipment-cards'
import { Card, CardContent } from '../components/ui/card'
import { currentIntlLocale, t } from '../i18n'
import { usePresentation } from '../presentation'
import type { InventoryArc, InventoryCartridge, InventoryModule, InventoryResource, WorkspaceCharacter } from '../types'

function normalizedKey(value: string) {
  return value.trim().toLocaleLowerCase()
}

function qualityName(value: string, qualities: Record<string, string>) {
  return qualities[normalizedKey(value)] || value || t('unknown_quality')
}

function qualityRank(value: string) {
  return ({ orange: 5, gold: 5, yellow: 5, purple: 4, violet: 4, blue: 3, green: 2, white: 1 }[normalizedKey(value)] || 0)
}

function geometryName(value: string, geometries: Record<string, string>) {
  return geometries[value] || value
}

function embeddedResourceAssetID(value: string) {
  return value.replaceAll('×', 'x')
}

function CollectionHeader({ title, count, query, onQuery }: { title: string; count: number; query: string; onQuery: (value: string) => void }) {
  return <header className="collection-header mb-7 flex flex-wrap items-end justify-between gap-4"><div><p className="eyebrow">{t('account_inventory')}</p><h1 className="text-4xl font-black tracking-tight sm:text-5xl">{title}</h1><p className="mt-2 text-slate-400">{count > 1 ? t('imported_many', { count }) : t('imported_one', { count })}</p></div><label className="relative"><Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-500" /><input className="h-10 w-72 rounded-lg border border-slate-700 bg-slate-950 pl-9 pr-3 text-sm outline-none focus:border-pink-500" placeholder={t('search_collection', { title: title.toLocaleLowerCase() })} value={query} onChange={event => onQuery(event.target.value)} /></label></header>
}

function findOwner(characterID: number | undefined, characters: WorkspaceCharacter[]): Owner | undefined {
  if (!characterID) return
  const character = characters.find(item => item.character_id === characterID)
  return { id: characterID, name: character?.name || t('owner_unknown') }
}

function itemMeta(quality: string, level: number, qualities: Record<string, string>, locked?: boolean) {
  return `${qualityName(quality, qualities)} · ${t('level')} ${level}${locked ? ` · ${t('locked')}` : ''}`
}

function CollectionFilter({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string }[]; onChange: (value: string) => void }) {
  return <label className="block min-w-0"><span className="mb-1.5 block text-[10px] font-black uppercase tracking-wider text-slate-400">{label}</span><select className="h-9 w-full min-w-0 rounded-md border-slate-700 bg-slate-950 px-3 text-xs font-semibold" value={value} onChange={event => onChange(event.target.value)}><option value="">{t('all')}</option>{options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>
}

function StatMultiFilter({ label, values, options, onChange }: { label: string; values: string[]; options: string[]; onChange: (values: string[]) => void }) {
  const { stats: labels } = usePresentation()
  const available = options.filter(option => !values.includes(option))
  return <div className="min-w-0"><span className="mb-1.5 block text-[10px] font-black uppercase tracking-wider text-slate-400">{label}</span><div className="stat-multi-filter flex min-h-9 items-center gap-1.5 overflow-hidden rounded-md border border-slate-700 bg-slate-950 px-1.5">{values.map(value => <button key={value} type="button" title={t('remove_filter')} onClick={() => onChange(values.filter(item => item !== value))} className="flex shrink-0 items-center gap-1 rounded-full border border-slate-700 bg-slate-800 px-2 py-0.5 text-[11px] font-bold text-slate-200">{labels[value] || value}<X className="size-3" /></button>)}<select aria-label={t('add_filter', { label: label.toLocaleLowerCase() })} className="h-8 min-w-24 flex-1 bg-transparent px-1 text-xs text-slate-300 outline-none" value="" onChange={event => { if (event.target.value) onChange([...values, event.target.value]) }}><option value="">{values.length ? t('add') : t('all')}</option>{available.map(option => <option key={option} value={option}>{labels[option] || option}</option>)}</select></div></div>
}

function CollectionFilters({ children, count, noun }: { children: ReactNode; count: number; noun: 'module' | 'cartouche' }) {
  const label = noun === 'module' ? (count === 1 ? t('one_module') : t('many_modules')) : (count === 1 ? t('one_cartridge') : t('many_cartridges'))
  return <div className="mb-4 rounded-xl border border-slate-800 bg-slate-900/60 p-3"><div className="grid items-end gap-3 lg:grid-cols-[minmax(10rem,.65fr)_minmax(15rem,1fr)_minmax(15rem,1fr)_auto]">{children}<span className="mb-1 inline-flex h-7 items-center justify-center whitespace-nowrap rounded-full bg-slate-800 px-2.5 text-[11px] font-bold text-slate-300">{count} {label}</span></div></div>
}

export function CartridgesPage({ items, characters }: { items: InventoryCartridge[]; characters: WorkspaceCharacter[] }) {
  const { stats: labels, qualities } = usePresentation()
  const [query, setQuery] = useState('')
  const [setID, setSetID] = useState('')
  const [mainFilters, setMainFilters] = useState<string[]>([])
  const [subFilters, setSubFilters] = useState<string[]>([])
  const needle = query.trim().toLocaleLowerCase()
  const sets = useMemo(() => Array.from(new Map(items.map(item => [item.set_id, item.set_name || item.set_id])).entries()).map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, currentIntlLocale())), [items])
  const mainOptions = useMemo(() => Array.from(new Set(items.flatMap(item => item.main_stats.map(stat => stat.property_id)))).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale())), [items, labels])
  const subOptions = useMemo(() => Array.from(new Set(items.flatMap(item => item.sub_stats.map(stat => stat.property_id)))).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale())), [items, labels])
  const visible = useMemo(() => items.filter(item => (!setID || item.set_id === setID) && mainFilters.every(property => item.main_stats.some(stat => stat.property_id === property)) && subFilters.every(property => item.sub_stats.some(stat => stat.property_id === property)) && `${item.set_name} ${item.set_id} ${[...item.main_stats, ...item.sub_stats].map(stat => labels[stat.property_id] || stat.property_id).join(' ')}`.toLocaleLowerCase().includes(needle)), [items, setID, mainFilters, subFilters, needle, labels])
  return <><CollectionHeader title={t('count_cartridges')} count={items.length} query={query} onQuery={setQuery} /><CollectionFilters count={visible.length} noun="cartouche"><CollectionFilter label={t('set')} value={setID} options={sets} onChange={setSetID} /><StatMultiFilter label={t('main_attributes')} values={mainFilters} options={mainOptions} onChange={setMainFilters} /><StatMultiFilter label={t('secondary_attributes')} values={subFilters} options={subOptions} onChange={setSubFilters} /></CollectionFilters><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{visible.map(item => <CartridgePieceCard key={item.local_id} item={item} title={item.set_name || item.set_id} meta={itemMeta(item.quality, item.level, qualities, item.locked)} owner={item.equipped_character_id ? '' : t('available_state')} ownerCharacter={findOwner(item.equipped_character_id, characters)} />)}{!visible.length && <EmptyCollection query={query || mainFilters.length || subFilters.length ? t('filters_active') : ''} />}</div></>
}

export function ModulesPage({ items, characters }: { items: InventoryModule[]; characters: WorkspaceCharacter[] }) {
  const { stats: labels, qualities, geometries: geometryLabels } = usePresentation()
  const [query, setQuery] = useState('')
  const [geometry, setGeometry] = useState('')
  const [mainFilters, setMainFilters] = useState<string[]>([])
  const [subFilters, setSubFilters] = useState<string[]>([])
  const needle = query.trim().toLocaleLowerCase()
  const geometries = useMemo(() => Array.from(new Set(items.map(item => item.geometry))).filter(Boolean).sort((a, b) => a.localeCompare(b, currentIntlLocale())).map(value => ({ value, label: geometryName(value, geometryLabels) })), [items, geometryLabels])
  const mainOptions = useMemo(() => Array.from(new Set(items.flatMap(item => item.main_stats.map(stat => stat.property_id)))).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale())), [items, labels])
  const subOptions = useMemo(() => Array.from(new Set(items.flatMap(item => item.sub_stats.map(stat => stat.property_id)))).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale())), [items, labels])
  const visible = useMemo(() => items.filter(item => (!geometry || item.geometry === geometry) && mainFilters.every(property => item.main_stats.some(stat => stat.property_id === property)) && subFilters.every(property => item.sub_stats.some(stat => stat.property_id === property)) && `${item.set_name} ${item.geometry} ${[...item.main_stats, ...item.sub_stats].map(stat => labels[stat.property_id] || stat.property_id).join(' ')}`.toLocaleLowerCase().includes(needle)), [items, geometry, mainFilters, subFilters, needle, labels])
  return <><CollectionHeader title={t('count_modules')} count={items.length} query={query} onQuery={setQuery} /><CollectionFilters count={visible.length} noun="module"><CollectionFilter label={t('shape')} value={geometry} options={geometries} onChange={setGeometry} /><StatMultiFilter label={t('main_attributes')} values={mainFilters} options={mainOptions} onChange={setMainFilters} /><StatMultiFilter label={t('secondary_attributes')} values={subFilters} options={subOptions} onChange={setSubFilters} /></CollectionFilters><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{visible.map(item => { const geometryLabel = geometryName(item.geometry, geometryLabels); return <ModulePieceCard key={item.local_id} item={item} title={item.set_name || t('module_label', { number: geometryLabel })} meta={`${geometryLabel} · ${item.area} ${item.area > 1 ? t('cells') : t('cell')} · ${itemMeta(item.quality, item.level, qualities, item.locked)}`} owner={item.equipped_character_id ? '' : t('available_state')} ownerCharacter={findOwner(item.equipped_character_id, characters)} /> })}{!visible.length && <EmptyCollection query={query || mainFilters.length || subFilters.length ? t('filters_active') : ''} />}</div></>
}

export function ArcsPage({ items }: { items: InventoryArc[] }) {
  const { qualities } = usePresentation()
  const [query, setQuery] = useState('')
  const needle = query.trim().toLocaleLowerCase()
  const visible = useMemo(() => items.filter(item => `${item.name} ${item.forkId} ${item.quality}`.toLocaleLowerCase().includes(needle)).sort((a, b) => b.level - a.level || qualityRank(b.quality) - qualityRank(a.quality) || (a.name || a.forkId).localeCompare(b.name || b.forkId, currentIntlLocale())), [items, needle])
  return <><CollectionHeader title={t('count_arcs')} count={items.length} query={query} onQuery={setQuery} /><div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{visible.map(item => <Card key={`${item.id.solt}:${item.id.serial}`} className="overflow-hidden"><CardContent className="flex items-center gap-4 pt-5"><AssetImage source={`/game_ui/forks/${encodeURIComponent(item.forkId)}.png`} label={item.name} className="size-20 object-contain" /><div className="min-w-0"><strong className="block text-lg">{item.name || item.forkId}</strong><small className="block text-slate-400">{itemMeta(item.quality, item.level, qualities)}</small><small className="block text-slate-400">{t('arc_details', { breakthrough: item.breakthrough, star: item.star })}</small>{item.equippedCharacterId ? <OwnerBadge owner={{ id: item.equippedCharacterId, name: item.equipped_character_name || t('owner_unknown') }} /> : <small className="block h-6 leading-6 text-emerald-400">{t('available_state')}</small>}</div></CardContent></Card>)}{!visible.length && <EmptyCollection query={query} />}</div></>
}

export function ResourcesPage({ items }: { items: InventoryResource[] }) {
  const [query, setQuery] = useState('')
  const needle = query.trim().toLocaleLowerCase()
  const visible = useMemo(() => items.filter(item => `${item.name} ${item.itemId}`.toLocaleLowerCase().includes(needle)).sort((a, b) => b.quantity - a.quantity || (a.name || a.itemId).localeCompare(b.name || b.itemId, currentIntlLocale())), [items, needle])
  return <><CollectionHeader title={t('count_resources')} count={items.length} query={query} onQuery={setQuery} /><div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{visible.map(item => <Card key={`${item.id.solt}:${item.id.serial}`} className="overflow-hidden"><CardContent className="flex items-center gap-4 pt-5"><AssetImage source={`/game_ui/resources/${encodeURIComponent(embeddedResourceAssetID(item.itemId))}.png`} label={item.name || item.itemId} className="size-20 object-contain" /><div className="min-w-0"><strong className="block break-words text-lg">{item.name || item.itemId}</strong><span className="mt-2 block text-2xl font-black tabular-nums text-slate-100">{item.quantity.toLocaleString(currentIntlLocale())}</span></div></CardContent></Card>)}{!visible.length && <EmptyCollection query={query} />}</div></>
}

function EmptyCollection({ query }: { query: string }) {
  return <Card className="col-span-full"><CardContent className="py-12 text-center text-slate-500">{query ? t('no_search_result', { query }) : t('no_imported_item')}</CardContent></Card>
}
