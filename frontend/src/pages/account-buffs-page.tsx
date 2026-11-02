import { useEffect, useState } from 'react'
import { AccountBuffs } from '../../wailsjs/go/main/DesktopApp'
import { t } from '../i18n'
import { usePresentation } from '../presentation'
import { formatStat } from '../lib/format'
import { Card, CardContent } from '../components/ui/card'
import type { AccountBuffPage, Stat } from '../types'

export function AccountBuffsPage({ locale }: { locale: string }) {
  const [page, setPage] = useState<AccountBuffPage>()
  const [error, setError] = useState(false)
  useEffect(() => {
    let active = true
    setPage(undefined)
    setError(false)
    AccountBuffs(locale).then(value => {
      if (active) setPage(value)
    }).catch(err => {
      if (active) setError(true)
      console.error('Failed to load account buffs', err)
    })
    return () => { active = false }
  }, [locale])
  const furniture = page?.furniture.filter(option => page.detected_furniture_ids?.includes(option.id)) ?? []
  const characters = page?.characters.filter(character => character.bond_bonus_active && character.bond_stats?.length) ?? []
  return <div className="grid gap-5">
    <header>
      <h1 className="text-3xl font-black">{t('nav_account_buffs')}</h1>
      <p className="mt-2 text-sm text-slate-400">{t('buff_information_help')}</p>
    </header>
    {error && <p role="alert" className="rounded-lg border border-red-900 p-3 text-sm text-red-200">{t('account_buffs_error')}</p>}
    {!page && !error && <p role="status">{t('account_buffs_loading')}</p>}
    {page && <>
      <section className="grid gap-3" aria-labelledby="furniture-buffs-title">
        <h2 id="furniture-buffs-title" className="text-lg font-bold">{t('account_furniture')}</h2>
        <div className="grid gap-3 md:grid-cols-3">
          {furniture.map(option => <BonusCard key={option.id} stats={option.stats} source={t('buff_source_scan_level', { level: option.level ?? '—' })} />)}
        </div>
        {!furniture.length && <p className="text-sm text-slate-400">{t('buff_no_furniture')}</p>}
      </section>
      <section className="grid gap-3" aria-labelledby="bond-buffs-title">
        <h2 id="bond-buffs-title" className="text-lg font-bold">{t('account_bond')}</h2>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {characters.map(character => <BonusCard key={character.character_id} name={character.name} stats={character.bond_stats!} source={t('buff_bond_source', { level: character.bond_level! })} />)}
        </div>
        {!characters.length && <p className="text-sm text-slate-400">{t('buff_no_bond')}</p>}
      </section>
    </>}
  </div>
}
function BonusCard({ name, stats, source }: { name?: string; stats: Stat[]; source: string }) {
  const { stats: labels } = usePresentation()
  return <Card><CardContent className="grid gap-2 pt-5">
    {name && <h3 className="font-bold">{name}</h3>}
    {stats.map(stat => <p key={stat.property_id} className="flex flex-wrap items-baseline justify-between gap-2">
      <span className="text-sm font-semibold">{labels[stat.property_id] || stat.property_id}</span>
      <strong className="text-xl text-emerald-300">+{formatStat(stat.property_id, stat.value)}</strong>
    </p>)}
    <p className="text-xs text-slate-400">{source}</p>
  </CardContent></Card>
}
