import { useState } from 'react'
import { ArrowDown, ArrowUp, Eye } from 'lucide-react'

import { currentIntlLocale, t } from '../i18n'
import { formatRanking, formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { Result } from '../types'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader } from './ui/card'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

type Sort = { key: string; direction: 1 | -1 }

export function BuildRankingTable({ builds, active, onSelect }: { builds: Result[]; active: number; onSelect: (rank: number) => void }) {
  const { stats: labels } = usePresentation()
  const columns = ['AtkFinal', 'CritBase', 'CritDamageBase', 'MagBase', 'DamageUpGeneralBase', 'DamageUpIncantationBase']
  const [sort, setSort] = useState<Sort>({ key: 'ranking', direction: -1 })
  const sortBy = (key: string) => setSort(current => current.key === key ? { key, direction: current.direction === 1 ? -1 : 1 } : { key, direction: key === 'set' || key === 'build' ? 1 : -1 })
  const sortValue = (build: Result, index: number, key: string): number | string => key === 'build' ? index : key === 'ranking' ? (build.solution.ranking?.score ?? -Infinity) : key === 'quality' ? build.solution.score : key === 'set' ? build.set.name : build.stats.derived[key] || 0
  const rows = builds.map((build, index) => ({ build, index })).sort((left, right) => {
    const a = sortValue(left.build, left.index, sort.key)
    const b = sortValue(right.build, right.index, sort.key)
    const compared = typeof a === 'string' && typeof b === 'string' ? a.localeCompare(b, currentIntlLocale()) : Number(a) - Number(b)
    return compared * sort.direction || left.index - right.index
  })
  const ranking = (builds[active] || builds[0])?.solution.ranking

  return <Card className="ranking-card"><CardHeader><p className="eyebrow">{t('results_section')}</p></CardHeader><CardContent><div className="table-scroll" tabIndex={0} aria-label={t('build_comparison')}><table className="ranking-table ranking-table--interactive"><thead><tr><SortableHeader label={t('build')} column="build" sort={sort} onSort={sortBy} /><SortableHeader label={t('ranking')} column="ranking" sort={sort} onSort={sortBy} />{columns.map(key => <SortableHeader key={key} label={labels[key] || key} column={key} sort={sort} onSort={sortBy} />)}<SortableHeader label={t('equipment_quality')} column="quality" sort={sort} onSort={sortBy} /><SortableHeader label={t('set')} column="set" sort={sort} onSort={sortBy} /></tr></thead><tbody>{rows.map(({ build, index }) => { const stats = build.stats.derived; const detail = build.solution.ranking; return <tr key={index} onClick={() => onSelect(index)} data-selected={active === index}><td><button aria-pressed={active === index} aria-label={t('build_number', { number: index + 1 })} className="rank-button" onClick={() => onSelect(index)}>#{String(index + 1).padStart(2, '0')}{active === index && <span aria-hidden="true">●</span>}</button></td><td className="ranking-score">{detail ? formatRanking(detail.score) : '—'}</td>{columns.map(key => <td key={key}>{formatStat(key, stats[key] || 0)}</td>)}<td>{formatRanking(build.solution.score)}</td><td className="ranking-set">{build.set.name || '—'}</td></tr> })}</tbody></table></div>{ranking && <ScoreDetailsDialog ranking={ranking} buildNumber={active + 1} />}</CardContent></Card>
}

function ScoreDetailsDialog({ ranking, buildNumber }: { ranking: NonNullable<Result['solution']['ranking']>; buildNumber: number }) {
  const { stats: labels } = usePresentation()
  return <div className="mt-4 flex justify-end"><Dialog><DialogTrigger asChild><Button variant="secondary"><Eye className="mr-2 size-4" />{t('why_score')}</Button></DialogTrigger><DialogContent><div className="pr-10"><h2 className="text-xl font-bold">{t('why_build_score', { number: String(buildNumber).padStart(2, '0') })}</h2><p className="mt-2 text-sm text-slate-400">{t('score_formula', { equipment: formatRanking(ranking.equipment), objectives: formatRanking(ranking.objectives), structure: formatRanking(ranking.structure) })}</p></div><div className="table-scroll mt-4"><table className="ranking-table"><thead><tr><th>{t('contribution')}</th><th>{t('build_value')}</th><th>{t('target')}</th><th>{t('linked_weight')}</th><th>{t('points')}</th></tr></thead><tbody>{(ranking.contributions || []).map(item => <tr key={`${item.property_id}-${item.source}-${item.input_property_id}`}><td>{labels[item.property_id] || item.property_id}{item.source && <small className="block text-slate-400">{item.source === 'main' ? t('primary_stats') : t('secondary_stats')}{item.input_property_id !== item.property_id ? ` · ${labels[item.input_property_id!] || item.input_property_id}` : ''}</small>}</td><td>{formatStat(item.input_property_id || item.property_id, item.value)}</td><td>{formatStat(item.property_id, item.target)}</td><td>{formatRanking(item.importance)}</td><td>{formatRanking(item.points)}</td></tr>)}<tr><td>{t('equipment_quality_detail')}</td><td colSpan={3}>{t('weighted_piece_scores')}</td><td>{formatRanking(ranking.equipment)}</td></tr><tr><td>{t('grid_set_bonus')}</td><td colSpan={3}>{t('grid_set_fill')}</td><td>{formatRanking(ranking.structure)}</td></tr><tr><td><strong>{t('total_ranking')}</strong></td><td colSpan={3} /><td><strong>{formatRanking(ranking.score)}</strong></td></tr></tbody></table></div></DialogContent></Dialog></div>
}

function SortableHeader({ label, column, sort, onSort }: { label: string; column: string; sort: Sort; onSort: (column: string) => void }) {
  const active = sort.key === column
  return <th aria-sort={active ? (sort.direction === 1 ? 'ascending' : 'descending') : 'none'}><button className="inline-flex items-center gap-1.5 whitespace-nowrap" onClick={() => onSort(column)}>{label}{active && (sort.direction === 1 ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />)}</button></th>
}
