import type { LucideIcon } from 'lucide-react'
import { Boxes, FolderInput, Package, Puzzle, Sparkles, Swords, Users } from 'lucide-react'

import { Button } from './ui/button'
import { t, type Locale } from '../i18n'
import { formatDateTime } from '../lib/format'
import type { AccountImportSummary, BuildWorkspace, EquipmentCatalog } from '../types'

export type Page = 'characters' | 'character-state' | 'builds' | 'cartridges' | 'modules' | 'arcs' | 'resources' | 'import'

type NavigationProps = {
  page: Page
  onPage: (page: Page) => void
}

type AppSidebarProps = NavigationProps & {
  characters: BuildWorkspace['characters']
  catalog: EquipmentCatalog
  importSummary: AccountImportSummary
  locale: Locale
  onLocaleChange: (value: Locale) => void
}

export function AppSidebar({ page, onPage, characters, catalog, importSummary, locale, onLocaleChange }: AppSidebarProps) {
  const saved = characters.filter(character => character.build).length
  return <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 overflow-x-hidden border-r border-slate-800 bg-[#12161e] p-5 lg:flex lg:flex-col">
    <div className="mb-8 min-w-0"><p className="eyebrow">{t('app_name')}</p><h1 className="mt-1 text-xl font-black">{t('build_planner')}</h1></div>
    <nav className="grid gap-2">
      <NavButton active={page === 'characters'} onClick={() => onPage('characters')} icon={Users} title={t('nav_characters')} detail={t('nav_available', { count: characters.length })} />
      <NavButton active={page === 'builds'} onClick={() => onPage('builds')} icon={Sparkles} title={t('nav_builds')} detail={t('nav_saved', { count: saved })} />
      <NavButton active={page === 'cartridges'} onClick={() => onPage('cartridges')} icon={Package} title={t('nav_cartridges')} detail={t('nav_owned', { count: catalog.cartridges.length })} />
      <NavButton active={page === 'modules'} onClick={() => onPage('modules')} icon={Puzzle} title={t('nav_modules')} detail={t('nav_owned', { count: catalog.modules.length })} />
      <NavButton active={page === 'arcs'} onClick={() => onPage('arcs')} icon={Swords} title={t('nav_arcs')} detail={t('nav_owned', { count: catalog.arcs.length })} />
      <NavButton active={page === 'resources'} onClick={() => onPage('resources')} icon={Boxes} title={t('nav_resources')} detail={t('nav_owned', { count: catalog.resources.length })} />
      <div className="my-2 border-t border-slate-800" />
      <NavButton active={page === 'import'} onClick={() => onPage('import')} icon={FolderInput} title={t('nav_import')} detail={importSummary.has_import ? formatDateTime(importSummary.imported_at) : t('nav_no_import')} />
    </nav>
    <div className="mt-auto border-t border-slate-800 pt-4"><LocaleSelector locale={locale} onChange={onLocaleChange} className="w-full" /></div>
  </aside>
}

export function MobileNavigation({ page, onPage, locale, onLocaleChange }: NavigationProps & { locale: Locale; onLocaleChange: (value: Locale) => void }) {
  const items: [Page, string, LucideIcon][] = [
    ['characters', 'nav_characters', Users],
    ['builds', 'nav_builds', Sparkles],
    ['cartridges', 'nav_cartridges', Package],
    ['modules', 'nav_modules', Puzzle],
    ['arcs', 'nav_arcs', Swords],
    ['resources', 'nav_resources', Boxes],
    ['import', 'nav_import', FolderInput],
  ]
  return <div className="mb-5 flex flex-wrap items-end gap-2 lg:hidden">
    <nav className="grid flex-1 grid-cols-2 gap-2 sm:grid-cols-3">{items.map(([id, key, Icon]) => <Button key={id} variant="secondary" className={page === id ? 'border-l-2 border-l-pink-600 bg-slate-800 text-slate-100' : ''} onClick={() => onPage(id)}><Icon className="mr-2 size-4" />{t(key)}</Button>)}</nav>
    <LocaleSelector locale={locale} onChange={onLocaleChange} className="w-24" />
  </div>
}

function LocaleSelector({ locale, onChange, className = '' }: { locale: Locale; onChange: (value: Locale) => void; className?: string }) {
  return <label className={`grid min-w-0 gap-1 text-[10px] font-bold uppercase tracking-wider text-slate-500 ${className}`}>
    <span>{t('language')}</span>
    <select className="h-8 w-full min-w-0 max-w-full rounded-md border border-slate-700 bg-slate-950 px-2 text-xs font-semibold normal-case tracking-normal text-slate-300" value={locale} onChange={event => onChange(event.target.value as Locale)}>
      <option value="fr">{t('language_fr')}</option><option value="en">{t('language_en')}</option>
    </select>
  </label>
}

function NavButton({ active, onClick, icon: Icon, title, detail }: { active: boolean; onClick: () => void; icon: LucideIcon; title: string; detail: string }) {
  return <button onClick={onClick} className={`flex items-center gap-3 rounded-r-xl border-y border-r p-3 text-left transition ${active ? 'border-y-slate-700 border-r-slate-700 border-l-2 border-l-pink-600 bg-slate-800/70 text-white' : 'border-transparent text-slate-400 hover:bg-slate-900 hover:text-white'}`}>
    <span className={`grid size-10 place-items-center rounded-lg ${active ? 'bg-slate-700 text-white' : 'bg-slate-900'}`}><Icon className="size-5" /></span>
    <span><strong className="block text-sm">{title}</strong><small className="text-slate-500">{detail}</small></span>
  </button>
}
