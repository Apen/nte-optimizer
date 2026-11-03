import { t } from '../i18n'
import { formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { GoalProgress, Result } from '../types'

export function StatsComparison({ result }: { result: Result }) {
  if (!result.goals?.length) return null
  return <section className="grid gap-4"><div className="overflow-x-auto rounded-xl border border-slate-800"><div className="min-w-[700px]"><div className="comparison-grid bg-slate-950/80 text-[10px] font-black uppercase tracking-wider text-slate-500"><span>{t('statistic')}</span><span>{t('in_game')}</span><span>{t('with_build')}</span><span>{t('objective_label')}</span><span>{t('result')}</span></div>{result.goals.map((goal, index) => <ComparisonRow key={goal.property_id} goal={goal} current={result.current_goals?.[index]} buffed={(result.stats.derived_with_all_buffs ?? result.stats.derived_conditional)?.[goal.property_id] ?? result.conditional_goals?.[index]?.current} currentBuffed={(result.current_stats?.derived_with_all_buffs ?? result.current_stats?.derived_conditional)?.[goal.property_id] ?? result.current_conditional_goals?.[index]?.current} />)}</div></div></section>
}

function signedStat(property: string, value: number) {
  return `${value > 0 ? '+' : value < 0 ? '−' : '±'}${formatStat(property, Math.abs(value))}`
}

function ComparisonRow({ goal, current, buffed, currentBuffed }: { goal: GoalProgress; current?: GoalProgress; buffed?: number; currentBuffed?: number }) {
  const { stats: labels } = usePresentation()
  const gameDelta = current ? goal.current - current.current : undefined
  const minimumDelta = goal.current - goal.minimum
  const maximumExceeded = goal.maximum != null && goal.maximum > 0 && goal.current > goal.maximum
  const objectiveDelta = maximumExceeded ? goal.current - goal.maximum! : minimumDelta
  function conditionalValue(base?: number, effect?: number) {
    if (base === undefined || effect === undefined) return null
    const value = formatStat(goal.property_id, effect)
    if (formatStat(goal.property_id, base) === value) return null
    return <span className="ml-1 whitespace-nowrap font-bold text-emerald-200" title={t('with_all_buffs_help')}>
      <span className="sr-only">{t('with_all_buffs')} </span>(<span aria-hidden="true">↑ </span>{value})
    </span>
  }
  return <div className="comparison-grid border-t border-slate-800 text-sm"><b>{labels[goal.property_id] || goal.label}</b><span>{current ? formatStat(goal.property_id, current.current) : '—'}{conditionalValue(current?.current, currentBuffed)}</span><span className="font-bold text-white">{formatStat(goal.property_id, goal.current)}{conditionalValue(goal.current, buffed)}</span><span>{formatStat(goal.property_id, goal.minimum)}</span><span className="grid justify-items-end gap-1.5"><b className={gameDelta === undefined ? 'badge-missing' : gameDelta >= 0 ? 'badge-ok' : 'badge-missing'}>{gameDelta === undefined ? t('in_game_unavailable') : t('in_game_result', { value: signedStat(goal.property_id, gameDelta) })}</b><b className={goal.reached ? 'badge-ok' : 'badge-missing'}>{maximumExceeded ? t('maximum_result', { value: signedStat(goal.property_id, objectiveDelta) }) : t('objective_result', { value: `${signedStat(goal.property_id, minimumDelta)}${goal.reached ? ' ✓' : ''}` })}</b></span></div>
}
