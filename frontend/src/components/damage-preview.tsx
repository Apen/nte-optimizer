import { currentIntlLocale, t } from '../i18n'
import type { Result } from '../types'

export function DamagePreview({ result }: { result: Result }) {
  const analysis = result.damage
  if (!analysis || analysis.status === 'unavailable') return null
  const rows = analysis.groups || []
	const hasScorch = rows.some(item => item.id === 'scorch')

  return <section className="grid gap-4"><div className="flex justify-end"><span className="rounded-full bg-fuchsia-950 px-3 py-1 text-xs font-bold text-fuchsia-300">{t('boss_badge', { level: analysis.enemy.level })}</span></div><div className="table-scroll"><table className="ranking-table min-w-[1050px]"><thead><tr><th>{t('action')}</th><th>{t('measure')}</th><th>{t('current_average')}</th><th>{t('current_crit')}</th><th>{t('build_average')}</th><th>{t('build_crit')}</th><th>{t('average_gain')}</th></tr></thead><tbody>{rows.map(item => {
    const periodic = item.category === 'dot' || item.category === 'reaction'
    const current = periodic ? (item.current_max_tick || 0) : (item.current_damage || 0)
    const build = periodic ? (item.build_max_tick || 0) : item.build_damage
    const currentCrit = periodic ? (item.current_max_crit_tick || 0) : (item.current_crit || 0)
    const buildCrit = periodic ? (item.build_max_crit_tick || 0) : item.build_crit
    const gain = current ? build / current - 1 : undefined
    const stacks = item.max_stacks || 1
    return <tr className={item.category === 'reaction' ? 'bg-orange-950/25' : item.category === 'dot' ? 'bg-fuchsia-950/20' : ''} key={item.id}><td><b className={`block ${item.category === 'reaction' ? 'text-orange-200' : item.category === 'dot' ? 'text-fuchsia-200' : ''}`}>{item.name} <span className="font-medium text-slate-400">({actionTypeLabel(item.action_type || item.category)})</span></b><small className="text-slate-500">{item.description}</small></td><td>{periodic ? <span><b>{t('tick')} · {stacks} {stacks > 1 ? t('stack_many') : t('stack_one')}</b><small className="block text-slate-500">{t('crit_fixed')}{item.category === 'reaction' ? ` · ${t('without_atk')}` : ''}</small></span> : <span>{t('full_action')}<small className="block text-slate-500">{item.instances} {item.instances > 1 ? t('component_many') : t('component_one')}</small></span>}</td><td className="font-semibold text-slate-400">{current ? Math.round(current).toLocaleString(currentIntlLocale()) : '—'}</td><td className="font-bold text-amber-200">{currentCrit ? Math.round(currentCrit).toLocaleString(currentIntlLocale()) : '—'}</td><td className="font-black">{Math.round(build).toLocaleString(currentIntlLocale())}</td><td className="font-black text-amber-200">{Math.round(buildCrit).toLocaleString(currentIntlLocale())}</td><td className={`font-bold ${gain === undefined ? 'text-slate-500' : gain >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{gain === undefined ? '—' : `${gain >= 0 ? '+' : ''}${(gain * 100).toFixed(1)} %`}</td></tr>
  })}</tbody></table></div>{hasScorch&&<p className="text-xs text-slate-500">{t('scorch_note')}</p>}<p className="text-xs text-slate-500">{t('enemy_note')}</p>{analysis.missing_inputs?.length && <p className="rounded-lg border border-amber-900/50 bg-amber-950/20 p-3 text-xs text-amber-200">{t('missing_damage_inputs', { inputs: analysis.missing_inputs.map(input => t(input)).join(', ') })}</p>}</section>
}

function actionTypeLabel(actionType:string) {
  const labels:Record<string,string>={ultimate:'skill_ultimate',skill:'skill_ability',basic_attack:'skill_basic_attack',qte:'skill_qte',charged_attack:'damage_type_charged_attack',dodge_counter:'damage_type_dodge_counter',plunging_attack:'damage_type_plunging_attack',dot:'damage_type_dot',reaction:'damage_type_reaction'}
  return t(labels[actionType]||'damage_type_other')
}
