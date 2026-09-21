import { currentIntlLocale, t } from '../i18n'
import type { Result } from '../types'
import { Card, CardContent, CardHeader } from './ui/card'

export function DamagePreview({ result }: { result: Result }) {
  const analysis = result.damage
  if (!analysis || analysis.status === 'unavailable') return null
  const rows = analysis.groups || []

  return <Card className="border-fuchsia-900/70"><CardHeader><div className="flex flex-wrap items-end justify-between gap-3"><div><p className="eyebrow">{t('damage_section')}</p><h2 className="text-2xl font-bold">{t('damage_title')}</h2><p className="mt-1 text-sm text-slate-400">{t('damage_description')}</p></div><span className="rounded-full bg-fuchsia-950 px-3 py-1 text-xs font-bold text-fuchsia-300">{t('boss_badge', { level: analysis.enemy.level })}</span></div></CardHeader><CardContent><div className="table-scroll"><table className="ranking-table min-w-[1050px]"><thead><tr><th>{t('action')}</th><th>{t('measure')}</th><th>{t('current_average')}</th><th>{t('current_crit')}</th><th>{t('build_average')}</th><th>{t('build_crit')}</th><th>{t('average_gain')}</th></tr></thead><tbody>{rows.map(item => {
    const periodic = item.category === 'dot' || item.category === 'reaction'
    const current = periodic ? (item.current_max_tick || 0) : (item.current_damage || 0)
    const build = periodic ? (item.build_max_tick || 0) : item.build_damage
    const currentCrit = periodic ? (item.current_max_crit_tick || 0) : (item.current_crit || 0)
    const buildCrit = periodic ? (item.build_max_crit_tick || 0) : item.build_crit
    const gain = current ? build / current - 1 : undefined
    const stacks = item.max_stacks || 1
    return <tr className={item.category === 'reaction' ? 'bg-orange-950/25' : item.category === 'dot' ? 'bg-fuchsia-950/20' : ''} key={item.id}><td><b className={`block ${item.category === 'reaction' ? 'text-orange-200' : item.category === 'dot' ? 'text-fuchsia-200' : ''}`}>{item.name}</b><small className="text-slate-500">{item.description}</small></td><td>{periodic ? <span><b>{t('tick')} · {stacks} {stacks > 1 ? t('stack_many') : t('stack_one')}</b><small className="block text-slate-500">{t('crit_fixed')}{item.category === 'reaction' ? ` · ${t('without_atk')}` : ''}</small></span> : <span>{t('full_action')}<small className="block text-slate-500">{item.instances} {item.instances > 1 ? t('component_many') : t('component_one')}</small></span>}</td><td className="font-semibold text-slate-400">{current ? Math.round(current).toLocaleString(currentIntlLocale()) : '—'}</td><td className="font-bold text-amber-200">{currentCrit ? Math.round(currentCrit).toLocaleString(currentIntlLocale()) : '—'}</td><td className="font-black">{Math.round(build).toLocaleString(currentIntlLocale())}</td><td className="font-black text-amber-200">{Math.round(buildCrit).toLocaleString(currentIntlLocale())}</td><td className={`font-bold ${gain === undefined ? 'text-slate-500' : gain >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{gain === undefined ? '—' : `${gain >= 0 ? '+' : ''}${(gain * 100).toFixed(1)} %`}</td></tr>
  })}</tbody></table></div><p className="mt-3 text-xs text-slate-500">{t('scorch_note')}</p><p className="mt-2 text-xs text-slate-500">{t('enemy_note')}</p>{analysis.missing_inputs?.length && <p className="mt-3 rounded-lg border border-amber-900/50 bg-amber-950/20 p-3 text-xs text-amber-200">{t('missing_damage_inputs', { inputs: analysis.missing_inputs.map(input => t(input)).join(', ') })}</p>}</CardContent></Card>
}
