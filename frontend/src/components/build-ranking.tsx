import { useState } from 'react'
import { ArrowDown, ArrowUp, Eye } from 'lucide-react'

import { currentIntlLocale, t } from '../i18n'
import { formatRanking, formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { EquipmentScoreContribution, Result, TargetGoal } from '../types'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader } from './ui/card'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

type Sort = { key: string; direction: 1 | -1 }

export function buildResultColumns(goals: Pick<TargetGoal, 'property_id'>[], disabledGoals: string[], mainStats: string[]) {
  const seen = new Set<string>()
  return ['BasicDamageIndex', ...goals.filter(goal => !disabledGoals.includes(goal.property_id)).map(goal => goal.property_id), ...mainStats].filter(property => {
    if (seen.has(property)) return false
    seen.add(property)
    return true
  })
}

export function BuildRankingTable({ builds, active, onSelect, goals, disabledGoals, mainStats }: { builds: Result[]; active: number; onSelect: (rank: number) => void; goals: Pick<TargetGoal, 'property_id'>[]; disabledGoals: string[]; mainStats: string[] }) {
  const { stats: labels } = usePresentation()
  const columns = buildResultColumns(goals, disabledGoals, mainStats)
  const [sort, setSort] = useState<Sort>({ key: 'ranking', direction: -1 })
  const effectiveSort = ['build', 'ranking', 'quality', 'set', ...columns].includes(sort.key) ? sort : { key: 'ranking', direction: -1 as const }
  const sortBy = (key: string) => setSort(current => current.key === key ? { key, direction: current.direction === 1 ? -1 : 1 } : { key, direction: key === 'set' || key === 'build' ? 1 : -1 })
  const sortValue = (build: Result, index: number, key: string): number | string => key === 'build' ? index : key === 'ranking' ? (build.solution.ranking?.score ?? -Infinity) : key === 'quality' ? build.solution.score : key === 'set' ? build.set.name : build.stats.derived[key] || 0
  const rows = builds.map((build, index) => ({ build, index })).sort((left, right) => {
    const a = sortValue(left.build, left.index, effectiveSort.key)
    const b = sortValue(right.build, right.index, effectiveSort.key)
    const compared = typeof a === 'string' && typeof b === 'string' ? a.localeCompare(b, currentIntlLocale()) : Number(a) - Number(b)
    return compared * effectiveSort.direction || left.index - right.index
  })
  const selectedBuild = builds[active] || builds[0]
  const ranking = selectedBuild?.solution.ranking

  const columnLabel = (key: string) => key === 'BasicDamageIndex' ? t('basic_damage') : labels[key] || key
  return <Card className="ranking-card"><CardHeader><p className="eyebrow">{t('results_section')}</p></CardHeader><CardContent><div className="table-scroll" tabIndex={0} aria-label={t('build_comparison')}><table className="ranking-table ranking-table--interactive"><thead><tr><SortableHeader label={t('build')} column="build" sort={effectiveSort} onSort={sortBy} /><SortableHeader label={t('ranking')} column="ranking" sort={effectiveSort} onSort={sortBy} />{columns.map(key => <SortableHeader key={key} label={columnLabel(key)} column={key} sort={effectiveSort} onSort={sortBy} />)}<SortableHeader label={t('equipment_quality')} column="quality" sort={effectiveSort} onSort={sortBy} /><SortableHeader label={t('set')} column="set" sort={effectiveSort} onSort={sortBy} /></tr></thead><tbody>{rows.map(({ build, index }) => { const stats = build.stats.derived; const detail = build.solution.ranking; return <tr key={index} onClick={() => onSelect(index)} data-selected={active === index}><td><button aria-pressed={active === index} aria-label={t('build_number', { number: index + 1 })} className="rank-button" onClick={() => onSelect(index)}>#{String(index + 1).padStart(2, '0')}{active === index && <span aria-hidden="true">●</span>}</button></td><td className="ranking-score">{detail ? formatRanking(detail.score) : '—'}</td>{columns.map(key => <td key={key}>{formatStat(key, stats[key] || 0)}</td>)}<td>{formatRanking(build.solution.score)}</td><td className="ranking-set">{build.set.name || '—'}</td></tr> })}</tbody></table></div>{ranking && selectedBuild && <ScoreDetailsDialog build={selectedBuild} buildNumber={active + 1} />}</CardContent></Card>
}

function ScoreDetailsDialog({ build, buildNumber }: { build: Result; buildNumber: number }) {
  const { stats: labels } = usePresentation()
  const ranking = build.solution.ranking!
  return <div className="mt-4 flex justify-end"><Dialog><DialogTrigger asChild><Button variant="secondary"><Eye className="mr-2 size-4" />{t('why_score')}</Button></DialogTrigger><DialogContent className="max-h-[96vh] w-[min(1500px,96vw)]"><div className="pr-10"><h2 className="text-xl font-bold">{t('why_build_score', { number: String(buildNumber).padStart(2, '0') })}</h2></div><div className="table-scroll score-details-table mt-4"><table className="ranking-table"><thead><tr><th>{t('contribution')}</th><th>{t('build_value')}</th><th>{t('target')}</th><th>{t('linked_weight')}</th><th>{t('points')}</th></tr></thead><tbody><tr className="bg-slate-900/80"><td colSpan={5}><strong>{t('objective_fit_section')}</strong><small className="ml-2 text-slate-400">{t('objective_fit_detail')}</small></td></tr>{(ranking.contributions || []).map(item => { const ratio=item.target>0?item.value/item.target:0; return <tr key={`${item.property_id}-${item.source}-${item.input_property_id}`}><td>{labels[item.property_id] || item.property_id}{item.source && <small className="block text-slate-400">{item.source === 'main' ? t('primary_stats') : t('secondary_stats')}{item.input_property_id !== item.property_id ? ` · ${labels[item.input_property_id!] || item.input_property_id}` : ''}</small>}</td><td>{formatStat(item.input_property_id || item.property_id, item.value)}<small className="block text-slate-400">{t('target_progress', { percent: formatRanking(ratio*100) })}</small></td><td>{formatStat(item.property_id, item.target)}</td><td>{formatRanking(item.importance)}</td><td><strong>{formatRanking(item.points)}</strong><small className="block whitespace-nowrap text-slate-400">{objectiveFormula(ratio,item.importance,item.target,item.value,item.score_scale)}</small></td></tr> })}<tr className="bg-cyan-950/35 text-cyan-100"><td><strong>{t('objective_fit_subtotal')}</strong></td><td colSpan={3}>{t('objective_fit_subtotal_detail')}</td><td><strong>{formatRanking(ranking.objectives)}</strong></td></tr><tr className="bg-slate-900/80"><td colSpan={5}><strong>{t('equipment_relevance_section')}</strong><small className="ml-2 text-slate-400">{t('equipment_relevance_detail')}</small></td></tr>{build.modules.map((module,index)=><EquipmentScoreRow key={module.module.local_id} label={t('module_label',{number:index+1})} contributions={module.breakdown.contributions} score={module.breakdown.total} labels={labels}/>) }<EquipmentScoreRow label={t('score_cartridge')} detail={build.set.name||'—'} contributions={build.cartridge_breakdown?.contributions} score={build.solution.cartridge_score} labels={labels}/><tr><td>{t('score_set_bonus')}</td><td colSpan={3}>{t('set_relevance_detail', { set: build.set.name || '—' })}</td><td>{formatRanking(build.solution.set_bonus_score)}</td></tr><tr className="bg-cyan-950/35 text-cyan-100"><td><strong>{t('equipment_quality_detail')}</strong></td><td colSpan={3}>{t('equipment_relevance_subtotal_detail')}</td><td><strong>{formatRanking(ranking.equipment)}</strong></td></tr><tr><td>{t('equipment_tie_break')}</td><td colSpan={3}>{t('equipment_tie_break_detail')}</td><td>{(ranking.tie_break||0).toLocaleString(currentIntlLocale(),{minimumFractionDigits:6,maximumFractionDigits:6})}</td></tr><tr className="bg-pink-950/55 text-pink-100"><td><strong>{t('total_ranking')}</strong></td><td colSpan={3}>{t('total_ranking_detail')}</td><td><strong className="text-base">{ranking.score.toLocaleString(currentIntlLocale(),{minimumFractionDigits:6,maximumFractionDigits:6})}</strong></td></tr></tbody></table></div></DialogContent></Dialog></div>
}

export function objectiveFormula(ratio:number,importance:number,target?:number,value?:number,scale?:number) {
  if(scale && target!==undefined && value!==undefined) {
    const capped=Math.min(Math.max(value,0),target)
    return `10 × ${formatRanking(importance)} × ${formatRanking(capped)} ÷ (${formatRanking(scale)} + ${formatRanking(capped)})`
  }
  if(ratio<1)return `(${formatRanking(Math.max(0,ratio)*100)}%⁴) × ${formatRanking(importance)} × 10`
  return `[1 + min(${formatRanking((ratio-1)*100)}%, 25%) × 0.2] × ${formatRanking(importance)} × 10`
}

function EquipmentScoreRow({label,detail,contributions,score,labels}:{label:string;detail?:string;contributions?:EquipmentScoreContribution[];score:number;labels:Record<string,string>}) {
  return <tr><td><strong>{label}</strong>{detail&&<small className="block text-slate-400">{detail}</small>}</td><td colSpan={3}><div className="grid gap-1">{contributions?.length?contributions.map((item,index)=><small key={`${item.property_id}-${item.source}-${index}`} className="text-slate-400"><span className="text-slate-200">{labels[item.property_id]||item.property_id}</span> ({item.source==='main'?t('primary_stats'):t('secondary_stats')}): {formatStat(item.property_id,item.effective_value??item.value)} ÷ {formatStat(item.property_id,item.reference_value||1)} × {formatRanking(item.weight)} = <strong className="text-slate-200">{formatRanking(item.score)}</strong>{item.capped?` · ${t('capped_value')}`:''}</small>):<small className="text-slate-500">{t('no_weighted_contribution')}</small>}</div></td><td><strong>{formatRanking(score)}</strong></td></tr>
}

function SortableHeader({ label, column, sort, onSort }: { label: string; column: string; sort: Sort; onSort: (column: string) => void }) {
  const active = sort.key === column
  return <th aria-sort={active ? (sort.direction === 1 ? 'ascending' : 'descending') : 'none'}><button className="inline-flex items-center gap-1.5 whitespace-nowrap" onClick={() => onSort(column)}>{label}{active && (sort.direction === 1 ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />)}</button></th>
}
