import { t } from '../i18n'
import { formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { GoalProgress, Result } from '../types'
import { Card, CardContent, CardHeader } from './ui/card'

export function StatsComparison({ result }: { result: Result }) {
  if (!result.goals?.length) return null
  const reached = result.goals.filter(goal => goal.reached).length
  return <Card className="border-slate-700 shadow-[inset_0_2px_#ff4598]"><CardHeader><div className="flex flex-wrap items-end justify-between gap-3"><div><p className="eyebrow">{t('best_proposal')}</p><h2 className="text-2xl font-bold">{t('stats_after_optimization')}</h2><p className="mt-1 text-sm text-slate-400">{t('objectives_reached', { reached, total: result.goals.length })}</p></div>{result.stats.completeness === 'partial' && <span className="rounded-full bg-amber-950 px-3 py-1 text-xs font-bold text-amber-300">{t('estimate')}</span>}</div></CardHeader><CardContent><div className="overflow-x-auto rounded-xl border border-slate-800"><div className="min-w-[700px]"><div className="comparison-grid bg-slate-950/80 text-[10px] font-black uppercase tracking-wider text-slate-500"><span>{t('statistic')}</span><span>{t('in_game')}</span><span>{t('with_build')}</span><span>{t('objective')}</span><span>{t('result')}</span></div>{result.goals.map((goal, index) => <ComparisonRow key={goal.property_id} goal={goal} current={result.current_goals?.[index]} conditional={result.conditional_goals?.[index]} currentConditional={result.current_conditional_goals?.[index]} />)}</div></div></CardContent></Card>
}

function signedStat(property: string, value: number) {
  return `${value > 0 ? '+' : value < 0 ? '−' : '±'}${formatStat(property, Math.abs(value))}`
}

function ComparisonRow({ goal, current, conditional, currentConditional }: { goal: GoalProgress; current?: GoalProgress; conditional?: GoalProgress; currentConditional?: GoalProgress }) {
  const { stats: labels } = usePresentation()
  const gameDelta = current ? goal.current - current.current : undefined
  const minimumDelta = goal.current - goal.minimum
  const maximumExceeded = goal.maximum != null && goal.maximum > 0 && goal.current > goal.maximum
  const objectiveDelta = maximumExceeded ? goal.current - goal.maximum! : minimumDelta
  const conditionalValue = (base?: number, effect?: number) => base !== undefined && effect !== undefined && Math.abs(base - effect) > .00001 ? <small className="block text-[9px] text-slate-500">{t('max_effects_value', { value: formatStat(goal.property_id, effect) })}</small> : null
  return <div className="comparison-grid border-t border-slate-800 text-sm"><b>{labels[goal.property_id] || goal.label}</b><span>{current ? formatStat(goal.property_id, current.current) : '—'}{conditionalValue(current?.current, currentConditional?.current)}</span><span className="font-bold text-white">{formatStat(goal.property_id, goal.current)}{conditionalValue(goal.current, conditional?.current)}</span><span>{formatStat(goal.property_id, goal.minimum)}</span><span className="grid justify-items-end gap-1.5"><b className={gameDelta === undefined ? 'badge-missing' : gameDelta >= 0 ? 'badge-ok' : 'badge-missing'}>{gameDelta === undefined ? t('in_game_unavailable') : t('in_game_result', { value: signedStat(goal.property_id, gameDelta) })}</b><b className={goal.reached ? 'badge-ok' : 'badge-missing'}>{maximumExceeded ? t('maximum_result', { value: signedStat(goal.property_id, objectiveDelta) }) : t('objective_result', { value: `${signedStat(goal.property_id, minimumDelta)}${goal.reached ? ' ✓' : ''}` })}</b></span></div>
}
