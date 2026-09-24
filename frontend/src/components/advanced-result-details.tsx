import { Info } from 'lucide-react'

import { currentIntlLocale, t } from '../i18n'
import { formatRanking, formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { Result } from '../types'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

export function AdvancedResultDetails({ result }: { result: Result }) {
  const { stats: labels, stat_sources: sourceNames } = usePresentation()
  return <Dialog>
    <DialogTrigger asChild><Button variant="secondary"><Info aria-hidden="true" className="mr-2 size-4" />{t('understand_result')}</Button></DialogTrigger>
    <DialogContent title={t('understand_result')} className="max-h-[92vh] w-[min(1100px,96vw)]">
      <h2 className="pr-10 text-xl font-bold">{t('understand_result')}</h2>
      <div className="understand-result-content mt-4"><div className="grid gap-3 sm:grid-cols-3"><Detail title={t('weighted_module_score')} value={formatRanking(result.solution.module_score)} /><Detail title={t('weighted_cartridge_score')} value={formatRanking(result.solution.cartridge_score)} /><Detail title={t('set_bonus_score_detail')} value={formatRanking(result.solution.set_bonus_score)} /><Detail title={t('combinations_detail')} value={result.solution.visited_states.toLocaleString(currentIntlLocale())} /><Detail title={t('solution_quality_detail')} value={result.solution.complete ? t('best_solution_confirmed_detail') : t('best_solution_found_detail')} /><Detail title={t('protected_pieces_detail')} value={result.include_equipped ? t('protection_ignored_detail') : String(result.excluded_equipped)} /></div>{result.stats.completeness === 'partial' && <p className="rounded-lg border border-amber-900/60 bg-amber-950/30 p-3 text-xs text-amber-200">{t('estimated_stats')}</p>}<div className="grid gap-2">{Object.entries(result.stats.sources).map(([source, values]) => <div className="grid gap-2 rounded-lg bg-slate-950 p-3 text-xs md:grid-cols-[190px_1fr]" key={source}><b>{sourceNames[source] || t('other_bonus_fallback')}</b><span className="text-slate-400">{Object.entries(values).map(([key, value]) => `${labels[key] || key} ${formatStat(key, value)}`).join(' · ') || '—'}</span></div>)}</div></div>
    </DialogContent>
  </Dialog>
}

function Detail({ title, value }: { title: string; value: string }) {
  return <div><p className="text-xs text-slate-500">{title}</p><strong>{value}</strong></div>
}
